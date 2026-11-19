package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

// A group's failure rests nobody: the doc's 组内无冷却. A key that
// answers 401 is tried again next request — the entries are the user's
// own list, and a chain that quietly benched them would look like the
// group ignoring it. A chain candidate never writes a rest (c.chain), so
// the same broken entry is asked on the next request and the request
// falls through to the one after it.
func TestChainGroupFailureRestsNobody(t *testing.T) {
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
	var badHits, goodHits atomic.Int32
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		badHits.Add(1)
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer good.Close()
	if err := provider.Save(provider.Provider{ID: "p1", Name: "P1", Chat: bad.URL, Key: "sk-1", Preset: "own"}); err != nil {
		t.Fatal(err)
	}
	if err := provider.Save(provider.Provider{ID: "p2", Name: "P2", Chat: good.URL, Key: "sk-2", Preset: "own"}); err != nil {
		t.Fatal(err)
	}
	// the chain: the broken entry first, the good one after it
	plan := &provider.Node{Kind: provider.KindRace, OrderBy: provider.OrderList, Seats: 1, MaxInFlight: 1, TotalWaitMs: 5000,
		Members: []provider.Node{
			{Kind: provider.KindEntry, Ref: "p1/m1", Cred: provider.KeyID("sk-1")},
			{Kind: provider.KindEntry, Ref: "p2/m1", Cred: provider.KeyID("sk-2")},
		}}
	if err := provider.SaveGroup(provider.Group{ID: "rc", Name: "RC", Plan: plan}); err != nil {
		t.Fatal(err)
	}
	s := New()
	ask := func() int {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"rc","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	// the broken entry is asked, fails, the good one answers; the broken
	// one must not have been put to rest
	if code := ask(); code == 0 {
		t.Fatal("first ask did not run")
	}
	if _, resting := restOf(""); resting {
		t.Fatal("the group's failure rested the empty key")
	}
	// a second request asks the broken entry again (no rest skipped it)
	if code := ask(); code == 0 {
		t.Fatal("second ask did not run")
	}
	if badHits.Load() < 2 {
		t.Fatalf("the failed entry was skipped as resting: %d vendor hits, want 2", badHits.Load())
	}
	if goodHits.Load() < 2 {
		t.Fatalf("the good entry was not asked each time: %d hits, want 2", goodHits.Load())
	}
}