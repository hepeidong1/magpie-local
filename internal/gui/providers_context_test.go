package gui

// POST /api/provider/context: the Supplier list's 现上下文长度 edits one
// model's window in place — provider.SetContext saves it, the answer is
// the providers list as a save's is, and a 0 takes the user's window away
// (the provider's own, or WindowUnknown, answers again).

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

func TestProviderContextSetsAndClears(t *testing.T) {
	testenv.Sandbox(t)
	p := provider.Provider{ID: "pc", Name: "PC", Preset: "own", Chat: "https://pc.test/v1", Key: "sk-pc", Models: []string{"m1"}}
	if err := provider.Save(p); err != nil {
		t.Fatal(err)
	}
	post := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/provider/context", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		providerContext(w, r)
		return w
	}

	w := post(`{"id":"pc","model":"m1","context":300000}`)
	if w.Code != 200 {
		t.Fatalf("set: %d %s", w.Code, w.Body.String())
	}
	saved, err := provider.Find("pc")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Contexts["m1"] != 300000 {
		t.Fatalf("contexts = %v, want m1 300000", saved.Contexts)
	}
	// the answer is the providers list, the model's window as set
	var list struct {
		Providers []struct {
			ID     string `json:"id"`
			Models []struct {
				ID      string `json:"id"`
				Context int    `json:"context"`
			} `json:"models"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("answer: %v", err)
	}
	found := false
	for _, p := range list.Providers {
		if p.ID != "pc" {
			continue
		}
		for _, m := range p.Models {
			if m.ID == "m1" {
				found = m.Context == 300000
			}
		}
	}
	if !found {
		t.Fatalf("the answer's m1 window is not 300000: %s", w.Body.String())
	}

	// 0 takes it away: the provider's own window (none here, so 200K)
	if w := post(`{"id":"pc","model":"m1","context":0}`); w.Code != 200 {
		t.Fatalf("clear: %d %s", w.Code, w.Body.String())
	}
	saved, err = provider.Find("pc")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := saved.Contexts["m1"]; ok {
		t.Fatalf("context kept after clearing: %v", saved.Contexts)
	}
}
