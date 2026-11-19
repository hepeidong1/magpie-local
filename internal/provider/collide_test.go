package provider

import (
	"testing"

	"github.com/yetone/magpie/internal/settings"
	"github.com/yetone/magpie/internal/testenv"
)

// A routed name that is also a real group's id: the routed name is the top
// of the three levels (the doc's 路由模型), so the bare name is served by
// the chain, not swallowed by the group of the same slug — the gateway's
// grouped() asks GroupFor the bare name before FindGroup, and that used to
// redirect "GLM-5.3-flash" to the group glm-5.3-flash in its own chain, the
// chain never run: one group, no cascade.
func TestRoutedNameBeatsGroupIDSlugMatch(t *testing.T) {
	testenv.Sandbox(t)
	putP(t, Provider{ID: "amd", Name: "AMD", Chat: "https://amd.test/v1", Key: "k1", Models: []string{"GLM-5.3-Flash"}})
	putP(t, Provider{ID: "zz", Name: "ZZ", Chat: "https://zz.test/v1", Key: "k2", Models: []string{"glm-5.3-flash"}})
	if err := SaveGroup(Group{ID: "glm-5.3-flash", Name: "GLM-5.3-flash test", Members: []string{"zz/glm-5.3-flash"}}); err != nil {
		t.Fatal(err)
	}
	if err := SaveGroup(Group{ID: "glm-5.3-flash-amd", Name: "GLM-5.3-flash amd", Members: []string{"amd/GLM-5.3-Flash"}}); err != nil {
		t.Fatal(err)
	}
	if err := settings.Save(settings.Settings{
		Exposed: []settings.Exposure{{Name: "GLM-5.3-flash", Refs: []string{"group/glm-5.3-flash-amd", "group/glm-5.3-flash"}}},
	}); err != nil {
		t.Fatal(err)
	}
	if id, ok := GroupFor("GLM-5.3-flash"); ok {
		t.Errorf("GroupFor redirected the routed name to %q", id)
	}
	g, ms, ok := FindGroup("GLM-5.3-flash")
	if !ok || g.ID != "GLM-5.3-flash" || g.Plan == nil || g.Plan.Kind != "order" || len(g.Plan.Members) != 2 {
		t.Fatalf("chain = %+v ok=%v", g, ok)
	}
	if len(ms) != 2 || ms[0].ID != "group/glm-5.3-flash-amd" || ms[1].ID != "group/glm-5.3-flash" {
		t.Fatalf("ms = %+v", ms)
	}
	// the gateway's Resolve answers the bare name too, with the chain's
	// first served target's entry as its representative
	if p, model, ok := Resolve("GLM-5.3-flash"); !ok || p.ID != "amd" || model != "GLM-5.3-Flash" {
		t.Fatalf("Resolve(bare routed name) = %q, %q, %v", p.ID, model, ok)
	}
	if g, _, ok := FindGroup("group/glm-5.3-flash"); !ok || g.ID != "glm-5.3-flash" {
		t.Fatalf("the real group keeps its own ref: %+v ok=%v", g, ok)
	}
}
