package gateway

// A chain's entries are judged each by its own window: a request too big
// for one entry's context is skipped (its row marked Skip="context") and
// the next entry is tried. This is the whole of roomy's work in this
// build — there is no race face standing for a group; a group's request
// plans one candidate per entry, and each is judged alone.

import (
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

func roomyCands(t *testing.T) (small, big candidate) {
	t.Helper()
	s := provider.Provider{ID: "s", Name: "S", Preset: "own", Chat: "https://s.test/v1", Key: "k1", Models: []string{"m1"}, Contexts: map[string]int{"m1": 200000}}
	b := provider.Provider{ID: "b", Name: "B", Preset: "own", Chat: "https://b.test/v1", Key: "k2", Models: []string{"m2"}, Contexts: map[string]int{"m2": 1000000}}
	if err := provider.Save(s); err != nil {
		t.Fatal(err)
	}
	if err := provider.Save(b); err != nil {
		t.Fatal(err)
	}
	return candidate{p: s, model: "m1", rest: "s"}, candidate{p: b, model: "m2", rest: "b"}
}

// an entry whose window is past the request is skipped, the one big
// enough stays
func TestRoomySkipsAnEntryPastItsWindow(t *testing.T) {
	testenv.Sandbox(t)
	small, big := roomyCands(t)
	cs := []candidate{small, big}
	pl := planned{order: []Weighed{{ID: "s/m1"}, {ID: "b/m2"}}}
	// a body of about 900K bytes: ~225K tokens, past the 200K window,
	// well under the 1M one
	body := []byte(`{"model":"g/m","messages":[{"role":"user","content":"` + strings.Repeat("x", 900000) + `"}]}`)
	out, _, smallOut, smallW := roomy(cs, pl, provider.Chat, body)
	if len(smallOut) != 1 || smallOut[0].rest != "s" {
		t.Fatalf("small = %+v, want the 200K entry alone", smallOut)
	}
	if len(out) != 1 || out[0].rest != "b" {
		t.Fatalf("out = %+v, want the 1M entry alone", out)
	}
	if len(smallW) != 1 || smallW[0].Window != 200000 || smallW[0].Skip != "context" {
		t.Fatalf("smallW = %+v, want the 200K window told as skipped for context", smallW)
	}
}

// an entry whose window is unknown is kept: the request is tried on it
// and falls over from its own refusal rather than skipped on a guess
func TestRoomyKeepsAnEntryOfUnknownWindow(t *testing.T) {
	testenv.Sandbox(t)
	small, big := roomyCands(t)
	unknown := candidate{p: provider.Provider{ID: "u", Name: "U", Preset: "own", Chat: "https://u.test/v1", Key: "k3", Models: []string{"m3"}}, model: "m3", rest: "u"}
	cs := []candidate{small, unknown, big}
	pl := planned{order: []Weighed{{ID: "s/m1"}, {ID: "u/m3"}, {ID: "b/m2"}}}
	body := []byte(`{"model":"g/m","messages":[{"role":"user","content":"` + strings.Repeat("x", 900000) + `"}]}`)
	out, _, smallOut, _ := roomy(cs, pl, provider.Chat, body)
	if len(smallOut) != 1 || smallOut[0].rest != "s" {
		t.Fatalf("small = %+v, want the 200K entry alone", smallOut)
	}
	if len(out) != 2 || out[0].rest != "u" || out[1].rest != "b" {
		t.Fatalf("out = %+v, want the unknown and the 1M entries in the plan's order", out)
	}
}