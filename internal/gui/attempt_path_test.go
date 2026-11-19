package gui

import (
	"testing"

	"github.com/yetone/magpie/internal/gateway"
)

// TestAttemptPath:一次请求的尝试路径按计划顺序列出每个条目，成了的
// 是 ok，问了失败的是 fail（带原因），没问的是 skip（带为什么）。
func TestAttemptPath(t *testing.T) {
	rt := gateway.Route{
		Provider: "p1",
		Order: []gateway.Weighed{
			{Provider: "p1", Model: "m1", Who: "k1"},
			{Provider: "p2", Model: "m2", Who: "k2"},
			{Provider: "p3", Model: "m3", Who: "k3"},
		},
		Tries: []gateway.Try{
			{ID: "p1#k1", Model: "m1", Status: 401, Error: "bad key"},
			{ID: "p2#k2", Model: "m2", Status: 200},
		},
		Left: []gateway.Weighed{
			{Provider: "p4", Model: "m4", Skip: "context", Window: 8000},
			{Provider: "p5", Model: "m5", Skip: "image"},
		},
	}
	path := attemptPath(rt)
	if len(path) != 5 {
		t.Fatalf("path = %d steps, want 5: %+v", len(path), path)
	}
	want := []struct {
		prov, model, result, why string
	}{
		{"p1", "m1", "fail", "bad key"},
		{"p2", "m2", "ok", ""},
		{"p3", "m3", "skip", "not asked"},
		{"p4", "m4", "skip", "context"},
		{"p5", "m5", "skip", "image"},
	}
	for i, w := range want {
		got := path[i]
		if got.Provider != w.prov || got.Model != w.model || got.Result != w.result || got.Why != w.why {
			t.Errorf("step %d = %+v, want %+v", i, got, w)
		}
	}
	if path[0].Status != 401 {
		t.Errorf("the failed step lost its status: %+v", path[0])
	}
}
