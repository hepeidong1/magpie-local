package gateway

// 带图请求不过滤条目：整条链照走，第一条原样收到图片、上游以图片为由
// 拒（400 "does not support image input"），网关记住它并换下一条——
// "传图片报错了让它自己跳过"。第二条收到同一张图并答了。

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

func TestChainImageRefusalMovesOn(t *testing.T) {
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
	var textOnlyHits, seeingHits atomic.Int32
	var textOnlyGotImage atomic.Bool
	textOnly := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		textOnlyHits.Add(1)
		buf := make([]byte, 1<<20)
		n, _ := r.Body.Read(buf)
		textOnlyGotImage.Store(strings.Contains(string(buf[:n]), "image_url"))
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":{"message":"Failed to deserialize the JSON body: unknown variant `+"`image_url`"+`, expected `+"`text`"+`"}}`)
	}))
	defer textOnly.Close()
	var seeingGotImage atomic.Bool
	seeing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seeingHits.Add(1)
		buf := make([]byte, 1<<20)
		n, _ := r.Body.Read(buf)
		seeingGotImage.Store(strings.Contains(string(buf[:n]), "image_url"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"seeing answered"}}]}`)
	}))
	defer seeing.Close()
	if err := provider.Save(provider.Provider{ID: "topt", Name: "TextOnly", Chat: textOnly.URL, Key: "sk-t", Preset: "own"}); err != nil {
		t.Fatal(err)
	}
	if err := provider.Save(provider.Provider{ID: "seep2", Name: "See", Chat: seeing.URL, Key: "sk-s", Preset: "own"}); err != nil {
		t.Fatal(err)
	}
	plan := &provider.Node{Kind: provider.KindRace, OrderBy: provider.OrderList, Seats: 1, MaxInFlight: 1, TotalWaitMs: 5000,
		Members: []provider.Node{
			{Kind: provider.KindEntry, Ref: "topt/m1", Cred: provider.KeyID("sk-t")},
			{Kind: provider.KindEntry, Ref: "seep2/m1", Cred: provider.KeyID("sk-s")},
		}}
	if err := provider.SaveGroup(provider.Group{ID: "img2", Name: "IMG2", Plan: plan}); err != nil {
		t.Fatal(err)
	}
	s := New()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"img2","messages":[{"role":"user","content":[{"type":"text","text":"what is this?"},{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="}}]}]}`))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if body := rec.Body.String(); !strings.Contains(body, "seeing answered") {
		t.Fatalf("the next entry did not answer after the image refusal: %q", body)
	}
	if textOnlyHits.Load() != 1 {
		t.Fatalf("the text-only entry was asked %d times, want 1", textOnlyHits.Load())
	}
	if !textOnlyGotImage.Load() {
		t.Fatal("the text-only entry was not sent the image: the chain must send it and let the vendor refuse")
	}
	if !seeingGotImage.Load() {
		t.Fatal("the next entry did not get the image")
	}
	if seeingHits.Load() != 1 {
		t.Fatalf("the seeing entry was asked %d times, want 1", seeingHits.Load())
	}
}