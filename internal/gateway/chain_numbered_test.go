package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

// A group may name one provider and key twice — two of its models on the
// single key, whose rest ID is the provider for every entry. The plan
// gives each row its own number (Weighed.No), and each try carries the
// number of the row it was asked on (Try.No), so the requests log can tell
// the pair apart; matched by the shared ID alone, one try looked like both
// entries ("在同一供应商在前后" two lit at once in the log).
func TestChainEntriesNumberedPerRow(t *testing.T) {
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
	var badHits, goodHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 1<<10)
		n, _ := r.Body.Read(body)
		model := string(body[:n])
		switch {
		case strings.Contains(model, "m1"):
			badHits++
			w.WriteHeader(401)
			fmt.Fprint(w, `{"error":{"message":"model m1 is retired"}}`)
		default:
			goodHits++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			fmt.Fprint(w, `{"choices":[{"message":{"content":"from "}}]}`)
		}
	}))
	defer srv.Close()
	// one provider, one key, two models: both entries of the group name it
	if err := provider.Save(provider.Provider{ID: "one", Name: "One", Chat: srv.URL, Key: "sk-1", Preset: "own"}); err != nil {
		t.Fatal(err)
	}
	plan := &provider.Node{Kind: provider.KindRace, OrderBy: provider.OrderList, Seats: 1, MaxInFlight: 1, TotalWaitMs: 5000,
		Members: []provider.Node{
			{Kind: provider.KindEntry, Ref: "one/m1", Cred: provider.KeyID("sk-1")},
			{Kind: provider.KindEntry, Ref: "one/m2", Cred: provider.KeyID("sk-1")},
		}}
	if err := provider.SaveGroup(provider.Group{ID: "two", Name: "Two", Plan: plan}); err != nil {
		t.Fatal(err)
	}
	s := New()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"two","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (m1 fails over to m2)", rec.Code)
	}
	if badHits != 1 || goodHits != 1 {
		t.Fatalf("hits = %d/%d, want m1 tried once then m2 once", badHits, goodHits)
	}
	sts := s.Trace(t.Context(), 0, 0)
	if len(sts.Routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(sts.Routes))
	}
	rt := sts.Routes[0]
	// both entries are the same ID (the provider, the one key) — only the
	// numbers differ
	for i, w := range rt.Order {
		if w.ID != "one" {
			t.Fatalf("order[%d].id = %q, want one (the provider)", i, w.ID)
		}
		if w.No == 0 {
			t.Fatalf("order[%d] has no number", i)
		}
		if i > 0 && rt.Order[i].No == rt.Order[i-1].No {
			t.Fatalf("order rows share the number %d: the pair is not told apart", w.No)
		}
	}
	// each try names its own row by the number
	got := map[int]string{}
	for _, tr := range rt.Tries {
		got[tr.No] = tr.Model
	}
	if len(rt.Tries) != 2 {
		t.Fatalf("tries = %d, want 2", len(rt.Tries))
	}
	for _, tr := range rt.Tries {
		if tr.No == 0 {
			t.Fatalf("try %s has no number", tr.Model)
		}
		if got[tr.No] != tr.Model {
			t.Fatalf("try %s's number %d names another row (%s)", tr.Model, tr.No, got[tr.No])
		}
	}
}