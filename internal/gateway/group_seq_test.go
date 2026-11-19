package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

// A group whose plan is a race still fails over on the callers that ask
// one candidate at a time — the classifier's decision group, a drawing
// member, an embedding model. planSchedule folds a race step into ONE
// candidate carrying c.race, which only the serve loop's raceAttempt
// can run; those callers ask c.p/c.model — the first entry's credential
// — and never look at c.race, so a race group's second entry was never
// asked and its first entry's failure was the whole answer. The plan
// here is the synthesized one-runner tree: a two-key provider's member
// expands to one entry per key, in list order (provider/plan.go).

func TestGroupSeqRaceFailsOverToNextEntry(t *testing.T) {
	testenv.Sandbox(t)
	forgetSpeeds()
	defer forgetSpeeds()
	// the rests store is process-global and the sandbox only swaps HOME:
	// the failed first key rests during the ask — take what this test
	// rested back out, or the next test's plan sees it resting
	t.Cleanup(func() {
		restingUntil.Lock()
		defer restingUntil.Unlock()
		for k := range restingUntil.m {
			if strings.HasPrefix(k, "sq") {
				delete(restingUntil.m, k)
				delete(restingUntil.note, k)
			}
		}
	})
	var mu sync.Mutex
	var asked []string
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		mu.Lock()
		asked = append(asked, auth)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(auth, "sk-a") { // the first entry is down
			w.WriteHeader(503)
			fmt.Fprint(w, `{"error":{"message":"overloaded"}}`)
			return
		}
		fmt.Fprint(w, `{"answers":{"intent":{"choice":"simple","confidence":0.9}}}`)
	}))
	defer vendor.Close()
	// a decision-only provider on two keys
	p := provider.Provider{ID: "sq", Name: "SQ", Preset: "own", Decide: vendor.URL, Key: "sk-a",
		Keys: []provider.KeyAccount{{Key: "sk-b"}}}
	if err := provider.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := provider.SaveGroup(provider.Group{ID: "sg", Name: "SG", Routing: provider.Ordered,
		Members: []string{"sq/m1"}}); err != nil {
		t.Fatal(err)
	}
	v, ok, err := New().askDecisionGroup(5*time.Second, "group/sg", []string{"simple"}, before{}, false, "")
	if !ok {
		t.Fatal("group/sg did not read as a decision group")
	}
	// Without the fix this fails with the first entry's 503 (the folded
	// race candidate is what the path asked, and nothing after it).
	if err != nil {
		t.Fatalf("askDecisionGroup: %v: the race's first entry was the path's only try", err)
	}
	if v.Intent != "simple" {
		t.Fatalf("verdict = %+v, want the second entry's answer", v)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(asked) < 2 || !strings.Contains(asked[len(asked)-1], "sk-b") {
		t.Fatalf("asked = %v: the race's second entry must be asked after the first fails", asked)
	}
}

// The same failover on the embeddings/retrieval path (serveRetrieval):
// before the change the folded race candidate was its only try, so the
// first entry's failure was the whole answer and the second key was never
// asked. This is a real user-visible change: an embeddings call to a race
// group used to fail on the first key alone. Reverting embeddings.go's
// planGroupSeq back to planGroup fails this test.
func TestGroupSeqRetrievalFailsOverToNextEntry(t *testing.T) {
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
	// the rests store is process-global and the sandbox only swaps HOME:
	// the failed first key rests during the ask — take what this test
	// rested back out, or the next test's plan sees it resting
	t.Cleanup(func() {
		restingUntil.Lock()
		defer restingUntil.Unlock()
		for k := range restingUntil.m {
			if strings.HasPrefix(k, "se") {
				delete(restingUntil.m, k)
				delete(restingUntil.note, k)
			}
		}
	})
	var mu sync.Mutex
	var asked []string
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		mu.Lock()
		asked = append(asked, auth)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(auth, "sk-a") { // the first entry is down
			w.WriteHeader(429)
			fmt.Fprint(w, `{"error":{"message":"out of quota"}}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"index":0,"embedding":[0.5,0.25]}],"model":"m1","usage":{"prompt_tokens":2,"total_tokens":2}}`)
	}))
	defer vendor.Close()
	// a chat provider on two keys, sk-a first
	p := provider.Provider{ID: "se", Name: "SE", Preset: "own", Chat: vendor.URL, Key: "sk-a",
		Keys: []provider.KeyAccount{{Key: "sk-b"}}}
	if err := provider.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := provider.SaveGroup(provider.Group{ID: "eg", Name: "EG", Routing: provider.Ordered,
		Members: []string{"se/m1"}}); err != nil {
		t.Fatal(err)
	}
	h := New().Handler()
	req := httptest.NewRequest("POST", "/v1/embeddings", strings.NewReader(`{"model":"eg","input":"hi"}`))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	// Without the fix the folded race candidate is the path's only try, so
	// this comes back 429 with sk-b never asked.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q: the race's first entry was the retrieval path's only try", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "0.5") {
		t.Fatalf("the second key did not answer: %q", body)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(asked) < 2 || !strings.Contains(asked[len(asked)-1], "sk-b") {
		t.Fatalf("asked = %v: the race's second entry must be asked after the first fails", asked)
	}
}
