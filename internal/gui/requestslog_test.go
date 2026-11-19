package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/gateway"
	"github.com/yetone/magpie/internal/testenv"
)

// testRoute is a two-entry chain: p1/m1 answered, p2/m2 the one after.
// done true: p2/m2 failed too (the request is over). done false: p2/m2's
// try is still out — the request waits for its first token there.
func testRoute(done bool) gateway.Route {
	rt := gateway.Route{
		ID: 1, Model: "free", Time: time.Now(), Done: done,
		Order: []gateway.Weighed{
			{Provider: "p1", Model: "m1"},
			{Provider: "p2", Model: "m2"},
		},
		Tries: []gateway.Try{
			{ID: "p1", Model: "m1", Done: true, Status: 200},
		},
	}
	if done {
		rt.Tries = append(rt.Tries, gateway.Try{ID: "p2", Model: "m2", Done: true, Status: 500, Error: "boom"})
	} else {
		rt.Tries = append(rt.Tries, gateway.Try{ID: "p2", Model: "m2", Done: false})
	}
	return rt
}

func TestRequestsLogPaths(t *testing.T) {
	testenv.Sandbox(t)
	done := requestsLogRows([]gateway.Route{testRoute(true)})
	if len(done) != 1 {
		t.Fatalf("rows = %d, want 1", len(done))
	}
	r := done[0]
	if !r.Done {
		t.Fatalf("row = %+v, want done", r)
	}
	if len(r.Path) != 2 {
		t.Fatalf("path = %+v, want the two entries", r.Path)
	}
	if r.Path[0].Result != "ok" {
		t.Fatalf("path[0].result = %q, want ok", r.Path[0].Result)
	}
	if r.Path[1].Result != "fail" || r.Path[1].Why != "boom" {
		t.Fatalf("path[1] = %+v, want fail/boom", r.Path[1])
	}
	// the answered provider and model are the row's
	if r.Provider != "p1" || r.Answered != "m1" {
		t.Fatalf("answered = %q/%q, want p1/m1", r.Provider, r.Answered)
	}

	live := requestsLogRows([]gateway.Route{testRoute(false)})
	if len(live) != 1 {
		t.Fatalf("live rows = %d, want 1", len(live))
	}
	if live[0].Done {
		t.Fatal("the live request is marked done")
	}
	if live[0].Path[0].Result != "ok" {
		t.Fatalf("live path[0].result = %q, want ok", live[0].Path[0].Result)
	}
	if live[0].Path[1].Result != "waiting" {
		t.Fatalf("live path[1].result = %q, want waiting (its try is still out)", live[0].Path[1].Result)
	}
}

// TestRequestsLogPendingNotWaiting: an entry the chain has not reached yet
// is "pending", never "waiting" — only the entry whose try is out waits.
func TestRequestsLogPendingNotWaiting(t *testing.T) {
	testenv.Sandbox(t)
	rt := gateway.Route{
		ID: 3, Model: "free", Time: time.Now(), Done: false,
		Order: []gateway.Weighed{
			{Provider: "p1", Model: "m1"},
			{Provider: "p2", Model: "m2"},
			{Provider: "p3", Model: "m3"},
		},
		Tries: []gateway.Try{{ID: "p1", Model: "m1", Done: false}}, // the first is being asked
	}
	rows := requestsLogRows([]gateway.Route{rt})
	if len(rows) != 1 || len(rows[0].Path) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Path[0].Result != "waiting" {
		t.Fatalf("path[0] = %q, want waiting", rows[0].Path[0].Result)
	}
	for _, i := range []int{1, 2} {
		if rows[0].Path[i].Result != "pending" {
			t.Fatalf("path[%d] = %q, want pending (not reached)", i, rows[0].Path[i].Result)
		}
	}
}

// TestRequestsLogNewestFirst: the trace lists routes oldest first; the log
// shows the newest at the top.
func TestRequestsLogNewestFirst(t *testing.T) {
	testenv.Sandbox(t)
	a := testRoute(true)
	a.ID = 10
	b := testRoute(true)
	b.ID = 20
	rows := requestsLogRows([]gateway.Route{a, b})
	if len(rows) != 2 || rows[0].ID != 20 || rows[1].ID != 10 {
		t.Fatalf("rows ids = %v, want [20 10] (newest first)", []int64{rows[0].ID, rows[1].ID})
	}
}

// TestRequestsLogSkipsLeft: an entry left out before asking (context too
// long, an image it doesn't take) is "skip" with why.
func TestRequestsLogSkipsLeft(t *testing.T) {
	testenv.Sandbox(t)
	rt := gateway.Route{
		ID: 2, Model: "free", Time: time.Now(), Done: true,
		Order: []gateway.Weighed{{Provider: "p1", Model: "m1"}},
		Tries: []gateway.Try{{ID: "p1", Model: "m1", Done: true, Status: 200}},
		Left: []gateway.Weighed{
			{Provider: "p2", Model: "m2", Skip: "context"},
			{Provider: "p3", Model: "m3", Skip: "image"},
		},
	}
	rows := requestsLogRows([]gateway.Route{rt})
	if len(rows) != 1 || len(rows[0].Path) != 3 {
		t.Fatalf("rows = %+v, want one row with the answered + two skipped entries", rows)
	}
	if rows[0].Path[1].Result != "skip" || rows[0].Path[1].Why != "context" {
		t.Fatalf("path[1] = %+v, want skip/context", rows[0].Path[1])
	}
	if rows[0].Path[2].Result != "skip" || rows[0].Path[2].Why != "image" {
		t.Fatalf("path[2] = %+v, want skip/image", rows[0].Path[2])
	}
}

// TestRequestsLogJSON: the row serializes to the shape the page reads
// (id, done, path[]), and an empty trace answers [].
func TestRequestsLogJSON(t *testing.T) {
	testenv.Sandbox(t)
	if requestsLogRows(nil) == nil {
		t.Fatal("nil rows: the endpoint answers [] not null")
	}
	b, err := json.Marshal(requestsLogRows([]gateway.Route{testRoute(true)}))
	if err != nil {
		t.Fatal(err)
	}
	var decoded []requestLogJSON
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0].ID != 1 || len(decoded[0].Path) != 2 {
		t.Fatalf("decoded = %+v, want one row, two path steps", decoded)
	}
}

// TestRequestsLogEndpoint: the endpoint answers [] when no gateway is
// served (the common window-before-gateway case).
func TestRequestsLogEndpoint(t *testing.T) {
	testenv.Sandbox(t)
	mux := http.NewServeMux()
	requestsLogRoutes(mux)
	r := httptest.NewRequest("GET", "/api/requests-log", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := strings.TrimSpace(w.Body.String()); got != "[]" {
		t.Fatalf("body = %q, want [] (no gateway served)", got)
	}
}

// TestScrubReqID: an upstream error's "request_id: chatcmpl-…" tail is
// cut from what the log shows (OpenAI-compatible vendors append it), the
// reason before it kept.
func TestScrubReqID(t *testing.T) {
	testenv.Sandbox(t)
	if got := scrubReqID("the model is overloaded; request_id: chatcmpl-abc123"); got != "the model is overloaded" {
		t.Fatalf("scrub = %q, want the reason without the request id", got)
	}
	if got := scrubReqID("quota exceeded"); got != "quota exceeded" {
		t.Fatalf("scrub left a plain reason changed: %q", got)
	}
	if got := scrubReqID(""); got != "" {
		t.Fatalf("empty stays empty, got %q", got)
	}
	// a reason with request_id in the middle (not just at the end) is
	// cut at the first marker
	if got := scrubReqID("request_id: x; 429 too fast"); got != "" {
		t.Fatalf("leading request_id should leave nothing, got %q", got)
	}
}

// TestRequestsLogSameProviderKeyRepeated: a group's chain may name one
// provider and key twice — two models on one key, or every entry of a
// provider with a single key (its ID is the provider for each). Only its
// own row number (No) tells the entry that was asked from its neighbor;
// matched by ID alone, the same try would light both rows, "回答中" on
// the pair front and back — the 前后 both lit screenshot.
func TestRequestsLogSameProviderKeyRepeated(t *testing.T) {
	testenv.Sandbox(t)
	rt := gateway.Route{
		ID: 11, Model: "free", Time: time.Now(), Done: false,
		Order: []gateway.Weighed{
			{ID: "p1", No: 1, Provider: "p1", Model: "m1"},
			{ID: "p1", No: 2, Provider: "p1", Model: "m2"},
		},
		Tries: []gateway.Try{{ID: "p1", No: 1, Model: "m1", Done: false}}, // the first is being asked
	}
	rows := requestsLogRows([]gateway.Route{rt})
	if len(rows) != 1 || len(rows[0].Path) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Path[0].Result != "waiting" {
		t.Fatalf("path[0] = %q, want waiting (the entry being asked)", rows[0].Path[0].Result)
	}
	if rows[0].Path[1].Result != "pending" {
		t.Fatalf("path[1] = %q, want pending (its neighbor was not asked; the same ID matched it before)", rows[0].Path[1].Result)
	}

	// both tried, same ID, one failed then the next — the numbers keep each
	// row its own outcome
	failed := gateway.Route{
		ID: 12, Model: "free", Time: time.Now(), Done: true,
		Order: []gateway.Weighed{
			{ID: "p1", No: 1, Provider: "p1", Model: "m1"},
			{ID: "p1", No: 2, Provider: "p1", Model: "m2"},
		},
		Tries: []gateway.Try{
			{ID: "p1", No: 1, Model: "m1", Done: true, Status: 500, Error: "boom"},
			{ID: "p1", No: 2, Model: "m2", Done: true, Status: 200},
		},
	}
	rows = requestsLogRows([]gateway.Route{failed})
	if rows[0].Path[0].Result != "fail" || rows[0].Path[1].Result != "ok" {
		t.Fatalf("rows = %+v, want the first row fail and its neighbor ok", rows[0].Path)
	}
}

// TestRequestsLogMerge: the log is the day's history backed up by the live
// trace — a route in both keeps the live row (it is fresher, and still
// holds the in-flight ones), and the merged rows come out oldest first for
// the page to show newest at the top.
func TestRequestsLogMerge(t *testing.T) {
	testenv.Sandbox(t)
	hist := []gateway.Route{
		{ID: 1, Time: time.Now().Add(-time.Hour), Model: "free", Done: true, Status: 200},
		{ID: 2, Time: time.Now().Add(-30 * time.Minute), Model: "free", Done: true, Status: 429, CallerKeyName: "李阳"},
		{ID: 3, Time: time.Now().Add(-10 * time.Minute), Model: "free", Done: true, Status: 200, CallerKeyName: "贺培东"},
	}
	live := []gateway.Route{
		{ID: 2, Time: time.Now().Add(-30 * time.Minute), Model: "free", Done: false, CallerKeyName: "李阳"}, // still going: live wins
		{ID: 4, Time: time.Now(), Model: "free", Done: false},
	}
	rows := requestsLogRows(requestsLogMerge(live, hist))
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4 (history's three and the live one)", len(rows))
	}
	// newest first, as the page shows
	if rows[0].ID != 4 || rows[len(rows)-1].ID != 1 {
		t.Fatalf("ids = %v, want [4 3 2 1] (newest first)", []int64{rows[0].ID, rows[1].ID, rows[2].ID, rows[3].ID})
	}
	// id 2: the live row, not the history's — it is still going
	for _, r := range rows {
		if r.ID == 2 {
			if r.Done {
				t.Fatal("the live row of id 2 was replaced by the history's done row")
			}
			if r.Key != "李阳" {
				t.Fatalf("key of id 2 = %q, want 李阳 (the history row's)", r.Key)
			}
			break
		}
	}
	// history only rows keep their key
	found := false
	for _, r := range rows {
		if r.ID == 3 {
			found = r.Key == "贺培东"
			break
		}
	}
	if !found {
		t.Fatal("the history-only row id 3 lost its key")
	}
}

// TestRequestsLogSameInstantStable: requests heard in the same instant
// (concurrent — Time equal) must keep their order from one refresh to the
// next. The merge gathers its routes from a map, whose order is random, so
// a sort by Time alone lets the equal-time rows walk every poll — the
// "时间一模一样就一直乱排序" bug. The id (each request's own, counted one
// by one) settles the tie.
func TestRequestsLogSameInstantStable(t *testing.T) {
	testenv.Sandbox(t)
	now := time.Now()
	hist := []gateway.Route{
		{ID: 5, Time: now, Model: "free", Done: true, Status: 200},
		{ID: 3, Time: now, Model: "free", Done: true, Status: 200},
		{ID: 7, Time: now, Model: "free", Done: true, Status: 200},
	}
	live := []gateway.Route{
		{ID: 1, Time: now, Model: "free", Done: false},
		{ID: 6, Time: now, Model: "free", Done: false},
	}
	// the map order gathering is random: rerun the merge many times and
	// every round must agree on the same order
	var first []int64
	for i := 0; i < 50; i++ {
		rows := requestsLogRows(requestsLogMerge(live, hist))
		var ids []int64
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		if first == nil {
			first = ids
			continue
		}
		if !slices.Equal(ids, first) {
			t.Fatalf("merge round %d reordered the same-instant rows: %v then %v", i, first, ids)
		}
	}
	// and the order itself is by id, ties by Time equal
	if !slices.Equal(first, []int64{7, 6, 5, 3, 1}) {
		t.Fatalf("ids = %v, want [7 6 5 3 1] (newest first, same-instant by id descending)", first)
	}
}

// TestRequestsLogManyKeysOneAsked: 30 keys of one provider and model in a
// group, and only one of them is being asked. Only that one is "waiting";
// the rest are "pending". Before, they were matched by provider+model, so
// all 30 lit up as "waiting" (the bug in the screenshot).
func TestRequestsLogManyKeysOneAsked(t *testing.T) {
	testenv.Sandbox(t)
	var order []gateway.Weighed
	for i := 0; i < 30; i++ {
		order = append(order, gateway.Weighed{
			ID: "nvidia#" + string(rune('a'+i)), Provider: "nvidia", Model: "nemotron-3.5-super-vl-120b-a12b",
		})
	}
	rt := gateway.Route{
		ID: 9, Model: "Nemotron-3.5-Super-VL", Time: time.Now(), Done: false,
		Order: order,
		Tries: []gateway.Try{{ID: "nvidia#a", Model: "nemotron-3.5-super-vl-120b-a12b", Done: false}},
	}
	rows := requestsLogRows([]gateway.Route{rt})
	if len(rows) != 1 || len(rows[0].Path) != 30 {
		t.Fatalf("rows = %+v", rows)
	}
	waiting := 0
	for i, s := range rows[0].Path {
		if s.Result == "waiting" {
			waiting++
			if i != 0 {
				t.Fatalf("path[%d] is waiting, want only path[0] (the key being asked)", i)
			}
		}
	}
	if waiting != 1 {
		t.Fatalf("waiting = %d, want exactly 1 (only the key asked)", waiting)
	}
}
