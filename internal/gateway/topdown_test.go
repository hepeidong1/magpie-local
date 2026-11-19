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

// TestGroupTriesTopDownOneAtATime:用户要的最简路由——暴露名找到组，
// 从上到下依次尝试，一次只发一条，前一条失败才轮到下一条；跳过只看
// 上下文和图片；没有冷却、没有粘性、没有竞速。
func TestGroupTriesTopDownOneAtATime(t *testing.T) {
	testenv.Sandbox(t)
	forgetSpeeds()
	defer forgetSpeeds()
	if err := access.EnsureKey(); err != nil {
		t.Fatal(err)
	}
	key := access.PrimarySecret()

	var first, second atomic.Int32
	mk := func(id string, hits *atomic.Int32, down bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if down {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":{"message":"bad key"}}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"from `+id+`"}}]}`)
		}))
	}
	a := mk("a", &first, true)  // 列表第一条：坏了
	b := mk("b", &second, false) // 列表第二条：好的
	defer a.Close()
	defer b.Close()
	for id, url := range map[string]string{"a": a.URL, "b": b.URL} {
		if err := provider.Save(provider.Provider{ID: id, Name: strings.ToUpper(id), Chat: url, Key: "sk-" + id, Preset: "own"}); err != nil {
			t.Fatal(err)
		}
	}
	plan := &provider.Node{Kind: provider.KindRace, OrderBy: provider.OrderSpeed, Seats: 9, MaxInFlight: 9,
		TotalWaitMs: 20000, Members: []provider.Node{
			{Kind: provider.KindEntry, Ref: "a/m1"},
			{Kind: provider.KindEntry, Ref: "b/m1"},
		}}
	if err := provider.SaveGroup(provider.Group{ID: "top", Name: "TOP", Plan: plan}); err != nil {
		t.Fatal(err)
	}
	h := New().Handler()
	ask := func() string {
		req := httptest.NewRequest("POST", "/v1/chat/completions",
			strings.NewReader(`{"model":"TOP","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	// 第一条失败 → 轮到第二条；组里写的是竞速/9 席，但实际一条一条来
	if body := ask(); !strings.Contains(body, "from b") {
		t.Fatalf("did not fall through to the second entry: %q", body)
	}
	if got := first.Load(); got != 1 {
		t.Errorf("the first entry was asked %d times, want 1", got)
	}
	if got := second.Load(); got != 1 {
		t.Errorf("the second entry was asked %d times, want 1", got)
	}

	// 下一条消息重新从队首开始（无粘性）：第一条又被问，仍失败，再轮到第二条
	if body := ask(); !strings.Contains(body, "from b") {
		t.Fatalf("second message did not start over at the top: %q", body)
	}
	if got := first.Load(); got != 2 {
		t.Errorf("the top entry was asked %d times over two messages, want 2 (no sticky, no rest)", got)
	}
	if got := second.Load(); got != 2 {
		t.Errorf("the second entry was asked %d times, want 2", got)
	}
}
