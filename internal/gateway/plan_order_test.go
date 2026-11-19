package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/routes"
	"github.com/yetone/magpie/internal/testenv"
)

// A provider's own routing strategy orders nothing any more (the doc's
// §4.3): a direct request tries the model's keys and accounts in the
// list's order, whatever Routing says, and its rows claim no order.
// The key tests save a three-key provider routed as named — registering
// the strategy as the rotation plugin does, so it really is the live
// ordering — and ask for the plan; the accounts test says the same of an
// account provider's accounts; the group test asks a whole request.

func planThreeKeys(t *testing.T, routing string) (*Server, provider.Provider) {
	t.Helper()
	remove := routes.Register(routing, routes.Builtin[routing])
	t.Cleanup(remove)
	p := provider.Provider{ID: "p9", Name: "P9", Preset: "own", Chat: "https://p9.test/v1", Routing: routing}
	p.Key = "sk-a"
	p.Keys = []provider.KeyAccount{{Key: "sk-b"}, {Key: "sk-c"}}
	if err := provider.Save(p); err != nil {
		t.Fatal(err)
	}
	return New(), p
}

func TestPlanProviderKeepsKeyListOrder(t *testing.T) {
	testenv.Sandbox(t)
	s, p := planThreeKeys(t, provider.LeastUsed)
	// the second key served lately: under the old "usage" strategy this
	// alone sent it behind the others; the list's order must stand
	rest := "p9#" + provider.KeyID("sk-b")
	served(rest, rest, 1000)
	cs, _ := s.plan(p, "p9/m1", provider.Chat)
	if len(cs) != 3 {
		t.Fatalf("candidates = %d, want 3", len(cs))
	}
	for i, want := range []string{"sk-a", "sk-b", "sk-c"} { // the saved order, whatever Routing says
		if cs[i].p.Key != want {
			t.Fatalf("candidate %d is key %q, want the list order %q", i, cs[i].p.Key, want)
		}
	}
}

// the availability filtering is untouched by the ordering going: a key
// resting after a rate limit is still moved behind the ready ones
// (restLast), resting is not the strategy's job.
func TestPlanProviderStillMovesRestingKeyBack(t *testing.T) {
	testenv.Sandbox(t)
	s, p := planThreeKeys(t, provider.LeastUsed)
	now := time.Now()
	rest := "p9#" + provider.KeyID("sk-b")
	until := now.Add(time.Minute)
	restingUntil.Lock()
	restingUntil.m[rest] = until
	restingUntil.note[rest] = Rest{Why: failRate, Status: 429, Until: until, Key: rest}
	restingUntil.Unlock()
	// the rests store is process-global and the sandbox only swaps HOME:
	// take the entry back out, or the next test in the binary sees a key
	// resting that nothing rested
	t.Cleanup(func() {
		restingUntil.Lock()
		defer restingUntil.Unlock()
		delete(restingUntil.m, rest)
		delete(restingUntil.note, rest)
	})
	cs, _ := s.plan(p, "p9/m1", provider.Chat)
	if len(cs) != 3 {
		t.Fatalf("candidates = %d, want 3", len(cs))
	}
	for i, want := range []string{"sk-a", "sk-c", "sk-b"} { // ready ones in list order, the resting one last
		if cs[i].p.Key != want {
			t.Fatalf("candidate %d is key %q, want %q: the resting key must still go last", i, cs[i].p.Key, want)
		}
	}
}

// The accounts sibling of the key tests above: §4.3 puts keys and
// accounts alike in the list's order, and an account goes through the
// other half of the code (perKeyEvery's account branch, weighingOf's
// UsageAgent lookup). A Codex provider on its own sign-in, with a second
// account on in logins.json — the store AlsoOn reads, beside the agent's
// own — plans its own account first and the other after it, whatever
// Least used weighs and whatever the second has lately served.
func TestPlanProviderKeepsAccountListOrder(t *testing.T) {
	testenv.Sandbox(t)
	remove := routes.Register(provider.LeastUsed, routes.Builtin[provider.LeastUsed])
	t.Cleanup(remove)
	// write the store before anything reads it: its pages keep what they
	// last read (filememo), and this test's file is the only one
	logins := filepath.Join(filepath.Dir(provider.Path()), "logins.json")
	if err := os.MkdirAll(filepath.Dir(logins), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logins, []byte(`[{"agent":"codex","user":"Two@X","on":true,"seen":"2026-01-01T00:00:00Z","auth":{"a":1}}]`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := provider.Provider{ID: "p9", Name: "P9", Preset: "own", Chat: "https://p9.test/v1", Routing: provider.LeastUsed,
		Account: &provider.Account{Agent: "codex", User: "One@X", Plan: "plus"}}
	// not saved: an account provider is what a sign-in makes, not what
	// providers.json keeps (Save would ask it for an API key), and plan
	// goes on the provider it is handed

	// the second account served lately: under the old ordering this alone
	// sent it behind the others; the list's order must stand
	served("p9@two@x", "p9@two@x", 1000)
	cs, _ := New().plan(p, "p9/m1", provider.Chat)
	if len(cs) != 2 {
		t.Fatalf("candidates = %d, want the two accounts on", len(cs))
	}
	for i, want := range []string{"One@X", "Two@X"} {
		if cs[i].p.Account == nil {
			t.Fatalf("candidate %d is not an account: the accounts sibling of the list order went untested", i)
		}
		if cs[i].p.Account.User != want {
			t.Fatalf("candidate %d is account %q, want the list order %q", i, cs[i].p.Account.User, want)
		}
	}
}

// The direct plan's rows claim no order (the review's Important 1): the
// turn mark, the strategy name and the sunk mark are gone from the row
// itself — a resting key still waits
// at the back, that being availability, which the test above pins. Even
// a provider that saved Sink and a rate-limited key says nothing of a
// back: the sinks split went with the ordering (the doc's §4.3), and
// what the page tells of a rate-limited key is its rest.
func TestPlanDirectRowsClaimNoOrder(t *testing.T) {
	testenv.Sandbox(t)
	// in turn saved, with Sink on and one key lately rate limited: the
	// list's order still stands
	s, p := planThreeKeys(t, provider.Rotate)
	p.Sink = true
	if err := provider.Save(p); err != nil {
		t.Fatal(err)
	}
	sankBy := "p9#" + provider.KeyID("sk-b")
	until := time.Now().Add(time.Minute)
	restingUntil.Lock()
	restingUntil.m[sankBy] = until
	restingUntil.note[sankBy] = Rest{Why: failRate, Status: 429, Until: until, Key: sankBy}
	restingUntil.Unlock()
	t.Cleanup(func() {
		restingUntil.Lock()
		defer restingUntil.Unlock()
		delete(restingUntil.m, sankBy)
		delete(restingUntil.note, sankBy)
	})
	cs, pl := s.plan(p, "p9/m1", provider.Chat)
	if len(cs) != 3 || len(pl.order) != 3 {
		t.Fatalf("candidates = %d, rows = %d, want three of each", len(cs), len(pl.order))
	}
	for i, want := range []string{"sk-a", "sk-c", "sk-b"} { // ready ones in list order, the resting one last
		if cs[i].p.Key != want {
			t.Fatalf("candidate %d is key %q, want %q: the resting key goes last and nothing sinks", i, cs[i].p.Key, want)
		}
	}
}

// The group's account list holds nothing any more (the doc's §4.3, §5.2:
// a plan's entries pin their own credentials): heldByGroup used to fold
// Group.Accounts into the caller on every group request path, and
// allowedCandidates then left out the accounts and keys it did not name.
// A group whose Accounts names only one key of a two-key provider, asked
// end-to-end: every key of that provider is still tried — the named one
// answers, and the unnamed one ahead of it gets its turn to fail first.
// (The group is saved the old way, by fields: a plan group's Save clears
// Accounts at all, provider/plan.go.)
func TestGroupAccountListHoldsNothing(t *testing.T) {
	testenv.Sandbox(t)
	forgetSpeeds()
	defer forgetSpeeds()
	if err := access.EnsureKey(); err != nil {
		t.Fatal(err)
	}
	key := access.PrimarySecret()
	if key == "" {
		t.Fatal("no gateway key to ask with")
	}
	var mu sync.Mutex
	var asked []string
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.Header.Get("Authorization"))
		mu.Unlock()
		if strings.Contains(r.Header.Get("Authorization"), "sk-a") {
			w.WriteHeader(401)
			fmt.Fprint(w, `{"error":{"message":"bad key"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"answer B"}}]}`)
	}))
	defer vendor.Close()
	p := provider.Provider{ID: "hk", Name: "HK", Preset: "own", Chat: vendor.URL, Key: "sk-a", Keys: []provider.KeyAccount{{Key: "sk-b"}}}
	if err := provider.Save(p); err != nil {
		t.Fatal(err)
	}
	// the group names only key b, as a gateway key's account list names it
	if err := provider.SaveGroup(provider.Group{ID: "hg", Name: "HG", Routing: provider.Ordered,
		Members:  []string{"hk/m1"},
		Accounts: []string{"hk/" + provider.KeyID("sk-b")}}); err != nil {
		t.Fatal(err)
	}
	h := New().Handler()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"hg","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q: the group's account list is still holding the request", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "answer B") {
		t.Fatalf("the provider's second key did not answer: %q", body)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, want := range []string{"sk-a", "sk-b"} {
		found := false
		for _, h := range asked {
			if strings.Contains(h, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("key %q never reached the vendor (asked: %v): every account of the provider must still be tried", want, asked)
		}
	}
}

// The same stop on the classifier's own path: askDecisionGroup still
// deleted the candidates a decision group's Accounts left out (the
// review's R2 leftover, classify.go's askClassifier → this function on
// every classifier ask to a decision group). Tested by calling
// askDecisionGroup directly, not end-to-end: reaching it through a
// conversation would mean a rule with this group as the classifier plus
// a turn to route, and none of that is this path's code — and the
// package has no existing decision test to drive either way. The group's
// plan is the one runner, whose candidate carries the first entry's key;
// with the old delete reinstated that candidate is the one the list
// leaves out, the ask returns no candidate at all, and this fails.
func TestDecisionGroupAccountListHoldsNothing(t *testing.T) {
	testenv.Sandbox(t)
	forgetSpeeds()
	defer forgetSpeeds()
	var mu sync.Mutex
	var asked []string
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		mu.Lock()
		asked = append(asked, auth)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"answers":{"intent":{"choice":"simple","confidence":0.9}}}`)
	}))
	defer vendor.Close()
	// a decision-only provider (Decide, no chat URL) on two keys
	p := provider.Provider{ID: "dj", Name: "DJ", Preset: "own", Decide: vendor.URL, Key: "sk-a",
		Keys: []provider.KeyAccount{{Key: "sk-b"}}}
	if err := provider.Save(p); err != nil {
		t.Fatal(err)
	}
	// saved the old way, by fields (a plan group's Save clears Accounts):
	// the list names only key b, never the key the group's plan tries
	if err := provider.SaveGroup(provider.Group{ID: "dg", Name: "DG", Routing: provider.Ordered,
		Members:  []string{"dj/m1"},
		Accounts: []string{"dj/" + provider.KeyID("sk-b")}}); err != nil {
		t.Fatal(err)
	}
	v, ok, err := New().askDecisionGroup(5*time.Second, "group/dg", []string{"simple"}, before{}, false, "")
	if !ok {
		t.Fatal("group/dg did not read as a decision group")
	}
	if err != nil {
		t.Fatalf("askDecisionGroup: %v: the group's account list is still holding this path", err)
	}
	if v.Intent != "simple" {
		t.Fatalf("verdict = %+v, want the answer of the provider's first key", v)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(asked) == 0 || !strings.Contains(asked[0], "sk-a") {
		t.Fatalf("asked = %v: key a, unnamed in the group's Accounts, must still be asked", asked)
	}
}
