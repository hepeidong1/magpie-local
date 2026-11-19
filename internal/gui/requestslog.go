package gui

// The Requests log: every request the gateway is serving or has just
// served, one row each, from the routing trace (gateway.Trace) — the
// trace holds the route from the moment the request came in (order: the
// group's whole chain) and updates its tries live, so the page shows a
// request "answering" while it waits for a first token and paints each
// entry of the chain as it is tried.
//
// The page (assets/plugins/requests/client.js) asks every 500 ms. The
// trace keeps trafficKeep (60) routes; the day's disk history (data/routing)
// backs it up, so a request long done — or one that finished before the
// gateway restarted — is still listed (requestsLogRoutes reads both).

import (
	"cmp"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yetone/magpie/internal/gateway"
)

// requestLogJSON is one request's row in the log.
type requestLogJSON struct {
	ID     int64     `json:"id"`
	Agent  string    `json:"agent,omitempty"`
	Time   time.Time `json:"time"`
	Key    string    `json:"key,omitempty"`   // the caller's gateway key name
	Group  string    `json:"group,omitempty"` // the group asked (its id)
	Model  string    `json:"model"`           // the model as asked (the group's bare name)
	Done   bool      `json:"done"`
	Status int       `json:"status,omitempty"`
	Error  string    `json:"error,omitempty"`
	Millis int64     `json:"ms,omitempty"`
	TTFT   int64     `json:"ttft,omitempty"`
	Flow   int64     `json:"flow,omitempty"`
	Input  int       `json:"input,omitempty"`
	Output int       `json:"output,omitempty"`
	Cache  int       `json:"cache,omitempty"`
	// Provider is the provider that answered ("" before any has), Model
	// the answer's model; the Requests page shows them beside the group.
	Provider string `json:"answeredProvider,omitempty"`
	Answered string `json:"answeredModel,omitempty"`
	// Path is the group's whole chain, in the order it is tried: each
	// entry with what came of it ("ok" / "fail" / "waiting" / "skip" /
	// "pending").
	Path []requestLogStep `json:"path,omitempty"`
}

// requestLogStep is one entry of the chain: the provider model it was,
// and what came of it now.
type requestLogStep struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Who      string `json:"who,omitempty"` // the account or key
	Result   string `json:"result"`        // "ok" | "fail" | "waiting" | "skip" | "pending"
	Why      string `json:"why,omitempty"` // why it failed or was skipped
	Status   int    `json:"status,omitempty"`
	Millis   int64  `json:"ms,omitempty"`
}

// requestsLogRows builds the log rows from the trace: the routes, newest
// first (the trace lists them oldest first), each with its chain. A route
// that is still running marks the entry whose try is out "waiting"; ones
// it has not reached yet are "pending" — a chain tries one at a time, so
// the only entry actually waiting is the one being asked now.
//
func requestsLogRows(rt []gateway.Route) []requestLogJSON {
	// newest first: the trace appends as requests come in
	for i, j := 0, len(rt)-1; i < j; i, j = i+1, j-1 {
		rt[i], rt[j] = rt[j], rt[i]
	}
	out := make([]requestLogJSON, 0, len(rt))
	for _, r := range rt {
		row := requestLogJSON{
			ID: r.ID, Agent: r.Agent, Time: r.Time, Model: r.Model,
			Done: r.Done, Status: r.Status, Error: r.Error, Millis: r.Millis,
			TTFT: r.TTFT, Flow: r.Flow, Output: r.Output,
		}
		if r.Group != nil {
			row.Group = r.Group.ID
		}
		row.Key = r.CallerKeyName
		// the billable tries' tokens, summed: what the request took
		for _, u := range r.Usage {
			row.Input += u.Input
			row.Cache += u.CacheRead + u.CacheWrite
		}
		row.Path = requestLogPath(r)
		// the answer's provider and model: the try that said ok
		for _, t := range r.Tries {
			if t.Done && t.Status > 0 && t.Status < 400 {
				row.Provider, row.Answered = stepProvider(t.ID), t.Model
				break
			}
		}
		out = append(out, row)
	}
	return out
}

// stepProvider is a try's provider, cut at the first of its slot markers
// ("provider", "provider#key", "provider@user").
func stepProvider(id string) string {
	if i := indexAny(id, "#@"); i > 0 {
		return id[:i]
	}
	return id
}

// requestLogPath is a route's chain: every planned entry (Route.Order)
// with what came of it, then the ones left out before asking (Route.Left,
// with why). A try's row is matched to the entry it went to by the entry's
// own id — "provider#key" / "provider@user" / "provider" — the same id a
// try carries, so a group of many keys of one provider and model lights
// only the one key that was asked, not all of them. One that answered is
// "ok", one that failed "fail", one whose try is still out — the request
// waits for its first token on it — "waiting", one the request has not
// reached yet "pending", and one never asked because it can't take the
// request "skip".
func requestLogPath(rt gateway.Route) []requestLogStep {
	if len(rt.Order) == 0 && len(rt.Left) == 0 {
		return nil
	}
	// a try is its entry's own by the number the entry was given when it
	// was planned (Weighed.No, carried to the try): a group's chain may
	// name one provider and key twice — two models on one key, or every
	// entry of a provider with a single key, whose ID is the provider for
	// each — and only the number tells which try went to which row. Rows
	// and tries without one (a provider asked outside a plan) match by the
	// entry's own id, as before; a model asked twice keeps the last.
	byNo := map[int]gateway.Try{}
	byID := map[string]gateway.Try{}
	for _, t := range rt.Tries {
		if t.No > 0 {
			byNo[t.No] = t
		} else {
			byID[t.ID] = t
		}
	}
	type pm struct{ pid, model string }
	byPM := map[pm]gateway.Try{}
	for _, t := range rt.Tries {
		byPM[pm{stepProvider(t.ID), t.Model}] = t
	}
	find := func(w gateway.Weighed) (gateway.Try, bool) {
		if w.No > 0 {
			t, ok := byNo[w.No]
			return t, ok
		}
		if w.ID != "" {
			// an id is the entry's own ("provider#key" / "provider@user" /
			// "provider"): it names the exact credential, so a group of
			// many keys of one provider and model lights only the asked one
			t, ok := byID[w.ID]
			return t, ok
		}
		// an id missing: match by provider+model alone
		t, ok := byPM[pm{w.Provider, w.Model}]
		return t, ok
	}
	out := make([]requestLogStep, 0, len(rt.Order)+len(rt.Left))
	seen := map[string]bool{}
	seenKey := func(w gateway.Weighed) string {
		if w.No > 0 {
			return "no:" + strconv.Itoa(w.No)
		}
		if w.ID != "" {
			return "id:" + w.ID
		}
		return "pm:" + w.Provider + "\x00" + w.Model
	}
	for _, w := range rt.Order {
		seen[seenKey(w)] = true
		st := requestLogStep{Provider: w.Provider, Model: w.Model, Who: w.Who}
		if t, ok := find(w); ok {
			st.Status, st.Millis = t.Status, t.Millis
			switch {
			case !t.Done:
				// the try is still out: the request waits for its first
				// word on this entry
				st.Result = "waiting"
			case t.Status > 0 && t.Status < 400:
				st.Result = "ok"
			default:
				st.Result, st.Why = "fail", scrubReqID(cmp.Or(t.Error, t.Fail, "failed"))
			}
		} else {
			// in the plan, not asked yet: the chain has not reached it
			st.Result = "pending"
		}
		out = append(out, st)
	}
	for _, w := range rt.Left {
		if seen[seenKey(w)] {
			continue
		}
		why := cmp.Or(w.Skip, skipWhyOf(w), "left out")
		out = append(out, requestLogStep{Provider: w.Provider, Model: w.Model, Who: w.Who, Result: "skip", Why: why})
	}
	return out
}

// indexAny is strings.IndexAny's first hit of the runes, or -1.
func indexAny(s, chars string) int {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return i
			}
		}
	}
	return -1
}

// scrubReqID takes the vendor's internal request id out of an error the
// log shows: OpenAI-compatible vendors append "request_id: chatcmpl-…"
// to an error body, which is nothing the user asked for. The id's rest
// ("request_id: …") is cut without touching the reason before it.
func scrubReqID(s string) string {
	if s == "" {
		return ""
	}
	if i := strings.LastIndex(s, "request_id:"); i >= 0 {
		s = s[:i]
	}
	// the separator the id was appended after ("…; request_id: …") goes
	// too, and any space left at either end
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ";,· ")
	return strings.TrimSpace(s)
}

// requestsLogRoutes answers /api/requests-log: the gateway's live
// requests, newest first, from the routing trace — and, since the trace is
// memory-only and a restart empties it, today's done requests from the
// disk history back it up (data/routing/), so the log survives a gateway
// restart: a request finished an hour ago is still there, not gone with
// the trace. The two read together, the live row winning where both have
// the same request (it is the fresher one; live keeps the still-going).
func requestsLogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/requests-log", func(rw http.ResponseWriter, r *http.Request) {
		gw := served.Load()
		if gw == nil {
			writeJSON(rw, []requestLogJSON{})
			return
		}
		out := requestsLogRows(requestsLogMerge(gw.Trace(r.Context(), 0, 0).Routes, historyRoutes(time.Now())))
		writeJSON(rw, out)
	})
}

// requestsLogKeep is how many of today's history routes the log reads
// behind the live trace's: the log page shows recent requests, and the
// day's file may hold thousands.
const requestsLogKeep = 100

// historyRoutes is today's done routes from the disk history, newest last,
// read at most every few seconds: the page asks twice a second, and the
// file only grows. A route in both (the trace still has it) is left for
// the live merge to prefer.
var historyCache = struct {
	sync.Mutex
	at    time.Time
	routes []gateway.Route
}{}

func historyRoutes(now time.Time) []gateway.Route {
	historyCache.Lock()
	defer historyCache.Unlock()
	if time.Since(historyCache.at) < 3*time.Second && historyCache.routes != nil {
		return historyCache.routes
	}
	var out []gateway.Route
	if _, routes, _ := gateway.History(now.Format("2006-01-02")); len(routes) > 0 {
		// History's routes are oldest first, the newest historyMax kept;
		// the log reads the newest requestsLogKeep of them, newest last as
		// History returns them (a reversal joins them under the live's)
		if len(routes) > requestsLogKeep {
			routes = routes[len(routes)-requestsLogKeep:]
		}
		out = routes
	}
	historyCache.at, historyCache.routes = now, out
	return out
}

// requestsLogMerge is the log's routes, still oldest first: the history's
// done routes, then the live trace's on top (a request in both keeps the
// live row). A route only in history is one the trace has let go — done
// and trimmed, or finished before the gateway restarted.
func requestsLogMerge(live, hist []gateway.Route) []gateway.Route {
	byID := make(map[int64]gateway.Route, len(live)+len(hist))
	for _, h := range hist {
		byID[h.ID] = h
	}
	for _, l := range live {
		byID[l.ID] = l
	}
	out := make([]gateway.Route, 0, len(byID))
	for _, r := range byID {
		out = append(out, r)
	}
	// oldest first again: the page's rows show newest at the top. Ties in
	// Time go by the request's own id — two requests heard at the same
	// instant are ordered by who came first (ids count one by one), so the
	// same instant's rows stay put from one refresh to the next (a sort by
	// Time alone, over a slice gathered from a map, walked them every poll)
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Time.Equal(out[j].Time) {
			return out[i].Time.Before(out[j].Time)
		}
		return out[i].ID < out[j].ID
	})
	return out
}