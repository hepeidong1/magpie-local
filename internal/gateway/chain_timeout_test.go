package gateway

// A chain entry that takes longer than chainFirstTokenWait to its first
// content is let go as failed — the upstream connection is cancelled —
// and the next entry is asked. This is the 2-minute cap (shortened here
// for the test): 超时=断开上游、切下一条.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

func TestChainEntryTimesOutAndMovesOn(t *testing.T) {
	testenv.Sandbox(t)
	if err := access.EnsureKey(); err != nil {
		t.Fatal(err)
	}
	key := access.PrimarySecret()
	// shorten the cap so the test runs in a fraction of a second
	old := chainFirstTokenWait
	chainFirstTokenWait = 250 * time.Millisecond
	t.Cleanup(func() { chainFirstTokenWait = old })

	var cancelled atomic.Int32
	// the slow entry: accepts the request, then says nothing until the
	// client cancels — never a first token
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		cancelled.Add(1)
	}))
	defer slow.Close()
	var fastHits atomic.Int32
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fastHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"from fast"}}]}`)
	}))
	defer fast.Close()

	if err := provider.Save(provider.Provider{ID: "slowp", Name: "Slow", Chat: slow.URL, Key: "sk-s", Preset: "own"}); err != nil {
		t.Fatal(err)
	}
	if err := provider.Save(provider.Provider{ID: "fastp", Name: "Fast", Chat: fast.URL, Key: "sk-f", Preset: "own"}); err != nil {
		t.Fatal(err)
	}
	plan := &provider.Node{Kind: provider.KindRace, OrderBy: provider.OrderList, Seats: 1, MaxInFlight: 1, TotalWaitMs: 5000,
		Members: []provider.Node{
			{Kind: provider.KindEntry, Ref: "slowp/m1", Cred: provider.KeyID("sk-s")},
			{Kind: provider.KindEntry, Ref: "fastp/m1", Cred: provider.KeyID("sk-f")},
		}}
	if err := provider.SaveGroup(provider.Group{ID: "to", Name: "TO", Plan: plan}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req := httptest.NewRequest("POST", "/v1/chat/completions",
		strings.NewReader(`{"model":"to","stream":true,"messages":[{"role":"user","content":"hi"}]}`)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	New().Handler().ServeHTTP(rec, req)

	if body := rec.Body.String(); !strings.Contains(body, "from fast") {
		t.Fatalf("did not move on to the fast entry after the slow one timed out: %q", body)
	}
	if fastHits.Load() != 1 {
		t.Fatalf("fast entry asked %d times, want 1", fastHits.Load())
	}
	// the slow upstream's connection was cancelled (断开)
	deadline := time.Now().Add(2 * time.Second)
	for cancelled.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if cancelled.Load() == 0 {
		t.Fatal("the slow entry's upstream was not cancelled on timeout")
	}
}