package provider

import (
	"testing"

	"github.com/yetone/magpie/internal/settings"
	"github.com/yetone/magpie/internal/testenv"
)

// A routed model's name resolves as a group whose plan is the chain:
// entry targets as they are, group targets as that group's own plan, a
// target that names nothing now left out, a chain of nothing not served.
func TestExposedChain(t *testing.T) {
	testenv.Sandbox(t)
	if err := settings.Save(settings.Settings{
		Exposed: []settings.Exposure{
			{Name: "chain", Refs: []string{"b/m1", "group/gone"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	g, ms, ok := ExposedChain("Chain")
	if !ok {
		t.Fatal("the chain must be served")
	}
	if g.ID != "chain" || g.Name != "chain" || g.Plan == nil || g.Plan.Kind != "order" {
		t.Fatalf("g = %+v", g)
	}
	if len(g.Plan.Members) != 1 || g.Plan.Members[0].Kind != "entry" || g.Plan.Members[0].Ref != "b/m1" {
		t.Fatalf("members = %+v", g.Plan.Members)
	}
	if len(ms) != 1 || ms[0].ID != "b/m1" {
		t.Fatalf("ms = %+v", ms)
	}
	if err := settings.Save(settings.Settings{
		Exposed: []settings.Exposure{{Name: "chain", Refs: []string{"group/gone"}}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := ExposedChain("chain"); ok {
		t.Fatal("a chain of nothing must not be served")
	}
}

// The group finder falls back to the chains: FindGroup answers a routed
// name with the chain group, and a real group of the same id wins.
func TestFindGroupServesRoutedNames(t *testing.T) {
	testenv.Sandbox(t)
	if err := settings.Save(settings.Settings{
		Exposed: []settings.Exposure{{Name: "my-fast", Refs: []string{"b/m1"}}},
	}); err != nil {
		t.Fatal(err)
	}
	g, _, ok := FindGroup("my-fast")
	if !ok || g.ID != "my-fast" || g.Plan == nil || g.Plan.Kind != "order" {
		t.Fatalf("g = %+v ok=%v", g, ok)
	}
	if _, _, ok := FindGroup("group/my-fast"); !ok {
		t.Fatal("the name serves by its group-prefixed ref too")
	}
}
