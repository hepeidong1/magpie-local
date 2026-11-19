package gateway

// What a provider or a group saved as its strategy changes nothing any
// more (the doc's §4.3, and §7's acceptance: "Routing 改成任何值都不影响
// 顺序"). The strategies are registered here as the rotation plugin does,
// so a name that really is served is exercised too — the point is that
// neither the plan's order, nor the rows it claims, nor what the keeping
// does with an answered conversation depends on the saved value, nor on
// Sink.

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

// every saved name, the default, and one no strategy serves
var deadRoutings = []string{
	"", provider.Ordered, provider.Rotate, provider.LeastUsed, provider.Pace,
	provider.Weighted, provider.FreeFirst, provider.Fastest, provider.Cheapest,
	provider.Drain, provider.Manual, "nobody/serves-this",
}

func clearSticks() {
	sticks.Lock()
	defer sticks.Unlock()
	sticks.m = map[string]stick{}
	sticks.opening = map[string]stick{}
}

func TestSavedProviderStrategyChangesNothing(t *testing.T) {
	testenv.Sandbox(t)
	forgetSpeeds()
	defer forgetSpeeds()
	var wantKeys []string
	var wantWhy, wantFirst string
	var wantKept bool
	for i, routing := range deadRoutings {
		for _, sink := range []bool{false, true} {
			clearSticks()
			s, p := planThreeKeys(t, routing)
			p.Sink = sink
			if err := provider.Save(p); err != nil {
				t.Fatal(err)
			}
			cs, pl := s.plan(p, "p9/m1", provider.Chat)
			if len(cs) != 3 || len(pl.order) != 3 {
				t.Fatalf("routing %q: candidates = %d, rows = %d, want three of each", routing, len(cs), len(pl.order))
			}
			keys := make([]string, 0, len(cs))
			for _, c := range cs {
				keys = append(keys, c.p.Key)
			}
			// a conversation the second key answered: the keeping's verdict
			// and the order it leaves are the strategy's last way in
			scope := p.ID + "/m1"
			in := http.Header{}
			body := []byte(`{"model":"p9/m1","messages":[{"role":"user","content":"hi"}]}`)
			answered(stickKey(scope, in, body), cs[1], 0, Usage{})
			got, _, aff, _ := affine(scope, "", false, in, provider.Chat, body, body, cs, pl)
			first := ""
			if len(got) > 0 {
				first = got[0].p.Key
			}
			if i == 0 && !sink {
				wantKeys, wantWhy, wantKept, wantFirst = keys, aff.Why, aff.Kept, first
				continue
			}
			if !reflect.DeepEqual(keys, wantKeys) {
				t.Fatalf("routing %q sink %v: order %v, want the list's %v", routing, sink, keys, wantKeys)
			}
			if aff.Why != wantWhy || aff.Kept != wantKept || first != wantFirst {
				t.Fatalf("routing %q sink %v: keeping %q kept %v first %q, want %q kept %v first %q",
					routing, sink, aff.Why, aff.Kept, first, wantWhy, wantKept, wantFirst)
			}
		}
	}
}

func TestSavedGroupStrategyChangesNothing(t *testing.T) {
	testenv.Sandbox(t)
	forgetSpeeds()
	defer forgetSpeeds()
	for _, id := range []string{"ga", "gb"} {
		if err := provider.Save(provider.Provider{ID: id, Name: strings.ToUpper(id), Preset: "own", Chat: "https://" + id + ".test/v1", Key: "sk-" + id}); err != nil {
			t.Fatal(err)
		}
	}
	s := New()
	var want []string
	for i, routing := range deadRoutings {
		clearSticks()
		// an old group, saved before plans: no Plan, so SaveGroup keeps the
		// saved Routing and Sink, and the chain derives its entries from
		// its members. A plan group would have both wiped by savePlanGroup
		// (the doc's §6: a plan group's behaviour is its entries), and the
		// test would then assert nothing.
		g := provider.Group{ID: "sd", Name: "SD", Routing: routing, Sink: true, Members: []string{"ga/m1", "gb/m1"}}
		if err := provider.SaveGroup(g); err != nil {
			t.Fatal(err)
		}
		saved, ms, ok := provider.FindGroup("group/sd")
		if !ok {
			t.Fatalf("routing %q: the group was not found", routing)
		}
		if saved.Routing != g.Routing {
			t.Fatalf("routing %q was not saved (%q): the test would assert nothing", routing, saved.Routing)
		}
		cs, pl := s.planGroup(saved, ms, provider.Chat)
		if len(cs) != 2 || len(pl.order) != 2 {
			t.Fatalf("routing %q: candidates = %d, rows = %d, want the two entries", routing, len(cs), len(pl.order))
		}
		got := make([]string, 0, len(cs))
		for _, c := range cs {
			got = append(got, c.rest)
		}
		if i == 0 {
			want = got
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("routing %q: entries %v, want %v", routing, got, want)
		}
	}
}
