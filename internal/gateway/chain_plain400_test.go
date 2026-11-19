package gateway

// A vendor that answers a plain 400 "The request is invalid" — words
// retryable() knows none of — is asked and failed like any other: the
// chain moves on to the next entry. Before, hw.failed() was false for
// such a 400 (nothing in retryable's words matched), so the generic
// "failed, try the next" branch was skipped and the whole chain stopped
// at the first entry's 400 — a group of 12 answered nothing at all over
// one entry's "request is invalid" (小红书's note3 relay, the ZCode
// probe). The chain's rule is 报错即试下一条, whatever the words.

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

func TestChainPlain400Continues(t *testing.T) {
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
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":{"message":"The request is invalid. Check the request and try again."}}`)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"from good"}}]}`)
	}))
	defer good.Close()
	if err := provider.Save(provider.Provider{ID: "plain400", Name: "Plain", Chat: bad.URL, Key: "sk-bad", Preset: "own"}); err != nil {
		t.Fatal(err)
	}
	if err := provider.Save(provider.Provider{ID: "goodp", Name: "Good", Chat: good.URL, Key: "sk-good", Preset: "own"}); err != nil {
		t.Fatal(err)
	}
	plan := &provider.Node{Kind: provider.KindRace, OrderBy: provider.OrderList, Seats: 1, MaxInFlight: 1, TotalWaitMs: 5000,
		Members: []provider.Node{
			{Kind: provider.KindEntry, Ref: "plain400/m1", Cred: provider.KeyID("sk-bad")},
			{Kind: provider.KindEntry, Ref: "goodp/m1", Cred: provider.KeyID("sk-good")},
		}}
	if err := provider.SaveGroup(provider.Group{ID: "p400", Name: "P400", Plan: plan}); err != nil {
		t.Fatal(err)
	}
	s := New()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"p400","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if body := rec.Body.String(); !strings.Contains(body, "from good") {
		t.Fatalf("did not move on to the good entry after the plain 400: %q", body)
	}
	if badHits.Load() != 1 {
		t.Fatalf("the 400 entry was asked %d times, want 1", badHits.Load())
	}
	if goodHits.Load() != 1 {
		t.Fatalf("the good entry was not asked: %d hits, want 1", goodHits.Load())
	}
}