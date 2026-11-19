package gateway

import (
	"crypto/subtle"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/budget"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
	"github.com/yetone/magpie/internal/usage"
)

// The gateway listens on this computer alone and answers only requests
// that carry an enabled gateway key, from here as from anywhere else.

// Port is the gateway's port.
func Port() string {
	_, p, err := net.SplitHostPort(Addr())
	if err != nil {
		return strconv.Itoa(settings.DefaultPort)
	}
	return p
}

// listen listens on addr; under lnMu, or before serving.
func (s *Server) listen(addr string) (net.Listener, error) {
	return Listen(addr)
}

// Relisten moves the gateway to the port settings now say. Requests in
// flight finish.
func (s *Server) Relisten() error {
	s.lnMu.Lock()
	defer s.lnMu.Unlock()
	if s.ln == nil {
		return nil // not serving
	}
	to := Addr()
	if s.ln.Addr().String() == to {
		return nil
	}
	was := s.ln.Addr().String()
	// another port (Settings' port changed): the new one is taken up before
	// the old one lets go, so one the gateway can't have leaves it serving
	// where it was
	if _, p, _ := net.SplitHostPort(was); p != Port() {
		ln, err := s.listen(to)
		if err != nil {
			return err
		}
		old := s.ln
		s.ln = ln
		old.Close()
		return nil
	}
	// the port is the same, so the old one goes first
	s.ln.Close()
	ln, err := s.listen(to)
	if err != nil {
		if ln, _ = s.listen(was); ln == nil {
			return err
		}
		s.ln = ln
		return err
	}
	s.ln = ln
	return nil
}

// lanGuard: every request is answered only with an enabled gateway key,
// from this computer as from any other. What carries its own credential
// instead — a browser's preflight (corsGuard answers it), the image
// server's own links, the Claude subscription's MCP helper's callback —
// is let through to be judged by what it carries.
func lanGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ownCallback(r) {
			next.ServeHTTP(w, r)
			return
		}
		// a WSL distro's agent, at the address Windows has on its network
		// (wsl.go), is one of this computer's
		if fromWSL(r) {
			r = asFromLoopback(r)
		}
		// a browser's preflight carries no key, and CORS forbids putting
		// one on it: corsGuard, next, answers it (204 and the headers) with
		// nothing behind it reached, while the call it asks after still
		// needs the key
		if corsPreflight(r) {
			next.ServeHTTP(w, r)
			return
		}
		// a link the image server handed out opens with its own token
		// and no key (imageTokenAsked); the handler judges the token, and
		// refuses one that opens nothing as a request without a key
		if imageTokenAsked(r) {
			next.ServeHTTP(w, r)
			return
		}
		var ok bool
		r, ok = identifyCaller(w, r)
		if !ok {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// callbacks are the callback tokens of the Claude subscription runs now
// live (addCallback, dropCallback).
var callbacks = struct {
	sync.Mutex
	tokens map[string]bool
}{tokens: map[string]bool{}}

// addCallback makes token, a run's callback token (randomToken: 24 bytes
// from crypto/rand), one ownCallback takes, until dropCallback.
func addCallback(token string) {
	callbacks.Lock()
	callbacks.tokens[token] = true
	callbacks.Unlock()
}

func dropCallback(token string) {
	callbacks.Lock()
	delete(callbacks.tokens, token)
	callbacks.Unlock()
}

// ownCallback: r is the MCP helper of a Claude Code magpie runs handing a
// tool call back, on the path it was given — POST claudeMCPPath and the
// callback token of a run now live, nothing else. It is let in without a
// gateway key from wherever it comes: the helper calls the address the
// gateway listens on with a token of the run's, not a key. The token,
// unguessable and handed to the helper in a file only this user reads, is
// what lets it in: a wrong, spent or missing one, another method, or any
// other path is judged as every request is.
func ownCallback(r *http.Request) bool {
	if r.Method != http.MethodPost || r.URL.RawPath != "" {
		return false
	}
	token, ok := strings.CutPrefix(r.URL.Path, claudeMCPPath)
	if !ok || token == "" {
		return false
	}
	// while the magpie this one took the gateway over from finishes, a
	// token this one doesn't have may be one of that one's runs: it is
	// passed on there (forwardCallback), and that magpie judges it as it
	// would its own
	if forwarding() {
		return true
	}
	callbacks.Lock()
	defer callbacks.Unlock()
	found := false
	for t := range callbacks.tokens {
		if subtle.ConstantTimeCompare([]byte(t), []byte(token)) == 1 {
			found = true
		}
	}
	return found
}

// callerGuard also covers embedded handlers used by the web app and tests.
func callerGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if access.Caller(r.Context()).KeyID == "" && managedKey(r) {
			var ok bool
			r, ok = identifyCaller(w, r)
			if !ok {
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func identifyCaller(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	var who access.Identity
	ok := false
	var refused *access.Refused
	for _, k := range callerKeys(r) {
		var err error
		if who, err = access.Check(k); err == nil {
			ok = true
			break
		}
		if refused == nil {
			errors.As(err, &refused)
		}
	}
	if !ok {
		if refused != nil {
			// an expired key, or one its issuer no longer lets in, is told
			// so on every route
			log.Printf("refused %s %s from %s: %s", r.Method, r.URL.Path, r.RemoteAddr, refused.Why)
			writeError(w, provider.Chat, http.StatusUnauthorized, refused.Why)
			return r, false
		}
		if over, retry := tooManyAuthFails(); over {
			// A flood of wrong keys: stop answering each one (no 401 oracle,
			// no unbounded work) until the bucket refills. A valid key never
			// reaches here, so a real caller is not held.
			w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			writeError(w, provider.Chat, http.StatusTooManyRequests, "too many failed authentications; slow down and send an enabled magpie gateway key")
			return r, false
		}
		msg := refusedKey(callerKey(r))
		log.Printf("refused %s %s from %s: %s", r.Method, r.URL.Path, r.RemoteAddr, msg)
		writeError(w, provider.Chat, http.StatusUnauthorized, msg)
		return r, false
	}
	r = r.WithContext(access.WithIdentity(r.Context(), who))
	r.Header = r.Header.Clone()
	r.Header.Set("Authorization", "Bearer "+Token)
	for _, h := range []string{"x-api-key", "x-goog-api-key"} {
		if r.Header.Get(h) != "" {
			r.Header.Set(h, Token)
		}
	}
	if q := r.URL.Query(); q.Get("key") != "" {
		q.Set("key", Token)
		u := *r.URL
		u.RawQuery = q.Encode()
		r.URL = &u
	}
	return r, true
}

// accountOf is the subscription account p answers as, named as the Routing
// trace names it (Account.User: an email, a login), never by a token; ""
// for a key or a provider without an account (#557).
func accountOf(p provider.Provider) string {
	if p.Account == nil {
		return ""
	}
	return p.Account.User
}

func appendUsage(r *http.Request, rec usage.Record) {
	if rec.Session == "" {
		rec.Session = sessionOf(r.Header)
	}
	if rec.Subagent == "" {
		rec.Subagent, rec.ParentAgent = subagentOf(r.Header)
	}
	who := access.Caller(r.Context())
	rec.CallerKeyID, rec.CallerKeyName, rec.CallerIssuerID = who.KeyID, who.KeyName, who.IssuerID()
	rec.Local = local(r)
	if r.Context().Value(observedKey{}) != nil {
		rec.Watched = true
	}
	if !rec.IsProbe() {
		// a request an MCP tool call made: its row names the model (mcpUse)
		mcpUseOf(r.Context()).used(rec.Provider, rec.Model)
	}
	budget.Append(rec)
}

// sharedWith: the request came from beyond this computer carrying an
// enabled gateway key, as every request to the gateway must.
func sharedWith(r *http.Request) bool {
	return !local(r) && access.Caller(r.Context()).KeyID != ""
}

// callerKey is the API key a request carries, however its client sends one:
// the first of callerKeys, "" for none.
func callerKey(r *http.Request) string {
	if ks := callerKeys(r); len(ks) > 0 {
		return ks[0]
	}
	return ""
}

// callerKeys are the API keys a request carries, in the order they are
// read: Authorization's bearer token, x-api-key, x-goog-api-key, ?key=. A
// header that carries none ("Authorization: Bearer ", sent beside an
// x-api-key by a client with no token set) is passed over, and a client
// that sends two different values — a stale or placeholder token in
// Authorization (ANTHROPIC_AUTH_TOKEN left at magpie), the key in x-api-key
// — is let in by whichever is an enabled gateway key.
func callerKeys(r *http.Request) []string {
	var out []string
	add := func(v string) {
		if v = strings.TrimSpace(v); v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	a := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(a) >= 7 && strings.EqualFold(a[:7], "Bearer ") {
		a = a[7:]
	} else if strings.EqualFold(a, "Bearer") {
		a = ""
	}
	add(a)
	for _, h := range []string{"x-api-key", "x-goog-api-key"} {
		add(r.Header.Get(h))
	}
	add(r.URL.Query().Get("key"))
	return out
}

// managedKey: the request carries a named gateway key's form (sk-magpie-…),
// or a key's own value a user gave it, in any of the places a key is read.
func managedKey(r *http.Request) bool {
	return slices.ContainsFunc(callerKeys(r), access.Named)
}

// refusedKey says why a caller's key was turned away: none came, or the
// one that came (its last four characters, and only of a long one) is no enabled
// gateway key, so a client that drops its key is told apart from a wrong
// key (#545).
func refusedKey(key string) string {
	if key == "" {
		return "no API key was sent: send a magpie gateway key as Authorization: Bearer <key> (or x-api-key)"
	}
	which := "the API key sent"
	if len(key) >= 12 { // a short one isn't named, so as not to give most of it away
		which = "the API key ending in " + key[len(key)-4:]
	}
	return which + " is not an enabled magpie gateway key: it is disabled, removed or mistyped"
}

// fromBrowser: the request carries what a browser puts on every request
// and a page's script can't take off.
func fromBrowser(r *http.Request) bool {
	return r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" ||
		r.Header.Get("Sec-Fetch-Mode") != "" || strings.HasPrefix(r.UserAgent(), "Mozilla/")
}

// local is a request from this computer: one that reached the gateway on
// loopback, wasn't passed on by a proxy or tunnel running here.
func local(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback() && !proxied(r)
}

// forwardHeaders are what a proxy or tunnel adds to a request it passes
// on: cloudflared (Cf-Connecting-IP, and X-Forwarded-For beside it),
// ngrok, Tailscale serve and funnel, frp's and Caddy's and nginx's HTTP
// proxies. An agent calling magpie sends none of them.
var forwardHeaders = []string{"Forwarded", "X-Forwarded-For", "X-Real-IP", "Cf-Connecting-IP", "True-Client-IP", "Tailscale-User-Login"}

// proxied: the request came to loopback through a proxy or tunnel on this
// computer (#1022, znjhahaha: a Cloudflare Tunnel to 127.0.0.1:3425), so it
// is someone else's, from wherever the proxy reaches. MAGPIE_TRUST_PROXY=1
// says a proxy here signs its clients in itself, and keeps what it
// forwards local, as before.
func proxied(r *http.Request) bool {
	if trustProxy() {
		return false
	}
	return slices.ContainsFunc(forwardHeaders, func(h string) bool { return r.Header.Get(h) != "" })
}

func trustProxy() bool {
	v, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("MAGPIE_TRUST_PROXY")))
	return v
}
