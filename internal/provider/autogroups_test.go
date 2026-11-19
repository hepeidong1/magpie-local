package provider

import (
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/testenv"
)

// §4.4: magpie finds groups on its own no more. Two providers serving
// one model under the same name gave an auto-<model> group before; none
// is created or listed now, the read is fixed off, and the off path
// answers for the old ids: a request naming one goes to that model from
// one provider (AutoStandIn), which is how an agent still set to an
// auto group keeps working.
func TestAutoGroupsAreNeverFound(t *testing.T) {
	testenv.Sandbox(t)
	putP(t, Provider{ID: "x1", Name: "X1", Chat: "https://x1.test/v1", Key: "k1", Models: []string{"shared"}})
	putP(t, Provider{ID: "x2", Name: "X2", Chat: "https://x2.test/v1", Key: "k2", Models: []string{"shared"}})
	for _, g := range Groups() {
		if strings.HasPrefix(g.ID, "auto-") || strings.HasPrefix(g.ID, "auto/") {
			t.Fatalf("found group listed: %q", g.ID)
		}
	}
	if AutoGroupsOn() {
		t.Fatal("AutoGroupsOn says on; magpie never finds groups now")
	}
	if sid, ok := AutoStandIn("group/auto-shared"); !ok || sid != "x1/shared" {
		t.Fatalf("AutoStandIn(group/auto-shared) = %q, %v; want x1/shared, true", sid, ok)
	}
	if sid, ok := AutoStandIn("auto/shared"); !ok || sid != "x1/shared" {
		t.Fatalf("AutoStandIn(auto/shared) = %q, %v; want x1/shared, true", sid, ok)
	}
}

// A provider the user changed into their own (saved under the auto- id)
// is still theirs: it stays listed, and AutoStandIn leaves it alone —
// the fixed-off read only stops magpie from finding new ones.
func TestUserChangedAutoGroupStays(t *testing.T) {
	testenv.Sandbox(t)
	putP(t, Provider{ID: "x1", Name: "X1", Chat: "https://x1.test/v1", Key: "k1", Models: []string{"shared"}})
	putP(t, Provider{ID: "x2", Name: "X2", Chat: "https://x2.test/v1", Key: "k2", Models: []string{"shared"}})
	if err := SaveGroup(Group{ID: "auto-shared", Name: "Mine", Members: []string{"x1/shared", "x2/shared"}}); err != nil {
		t.Fatal(err)
	}
	var listed bool
	for _, g := range Groups() {
		if g.ID == "auto-shared" {
			listed = true
		}
	}
	if !listed {
		t.Fatal("the user's changed found group is not listed")
	}
	if _, ok := AutoStandIn("group/auto-shared"); ok {
		t.Fatal("AutoStandIn moves a request off the user's own group")
	}
}

// §5.1: a group's name in the agents' lists carries no "· routing
// group" suffix now — fixed, not configurable.
func TestGroupNameHasNoSuffix(t *testing.T) {
	testenv.Sandbox(t)
	putP(t, Provider{ID: "p1", Name: "P1", Chat: "https://p1.test/v1", Key: "sk", Models: []string{"m1"}})
	if err := SaveGroup(Group{ID: "g1", Name: "G1", Members: []string{"p1/m1"}}); err != nil {
		t.Fatal(err)
	}
	var e Entry
	found := false
	for _, x := range Served() {
		if x.Group == "g1" {
			e, found = x, true
		}
	}
	if !found {
		t.Fatal("the group has no catalog entry")
	}
	if got := e.Label(); got != "G1" {
		t.Fatalf("Label = %q, want the plain G1", got)
	}
	if got := Labels([]Entry{e})[0]; strings.Contains(got, "routing group") {
		t.Fatalf("Labels = %q, want no routing group suffix", got)
	}
	// models keep their provider's after them: only the group's went plain
	for _, x := range Served() {
		if x.Group == "" && x.Provider.ID == "p1" {
			if got := x.Label(); !strings.Contains(got, "· P1") {
				t.Fatalf("model Label = %q, want it to keep its provider", got)
			}
		}
	}
}

// A config written while the suffix was there names the group
// "G1 · routing group": the old suffixed form must still resolve to the
// group (GroupFor is what the gateway's routing asks before any
// provider/model lookup).
func TestGroupNameOldSuffixStillResolves(t *testing.T) {
	testenv.Sandbox(t)
	putP(t, Provider{ID: "p1", Name: "P1", Chat: "https://p1.test/v1", Key: "sk", Models: []string{"m1"}})
	if err := SaveGroup(Group{ID: "g1", Name: "G1", Members: []string{"p1/m1"}}); err != nil {
		t.Fatal(err)
	}
	for _, asked := range []string{"G1 · routing group", "G1· routing group", "G1 · ROUTING GROUP", "G1 · routing group[1m]"} {
		if gid, ok := GroupFor(asked); !ok || gid != "group/g1" {
			t.Fatalf("GroupFor(%q) = %q, %v; want group/g1, true", asked, gid, ok)
		}
	}
}

// The fallback tries the name as given before the old-suffix cut: with a
// group literally named "X · routing group" and another named "X", the
// full name is the literal one's, not a route to the plain "X" through
// the cut. (Swap the two groupNamedBy calls in groupNamed and this fails
// — the ask strips to "X" and takes the wrong group.)
func TestGroupLiteralSuffixedNameWinsOverPlain(t *testing.T) {
	testenv.Sandbox(t)
	putP(t, Provider{ID: "p1", Name: "P1", Chat: "https://p1.test/v1", Key: "sk", Models: []string{"m1"}})
	if err := SaveGroup(Group{ID: "plain", Name: "X", Members: []string{"p1/m1"}}); err != nil {
		t.Fatal(err)
	}
	if err := SaveGroup(Group{ID: "literal", Name: "X · routing group", Members: []string{"p1/m1"}}); err != nil {
		t.Fatal(err)
	}
	if gid, ok := GroupFor("X · routing group"); !ok || gid != "group/literal" {
		t.Fatalf("GroupFor(%q) = %q, %v; want group/literal, true", "X · routing group", gid, ok)
	}
	if gid, ok := GroupFor("X"); !ok || gid != "group/plain" {
		t.Fatalf("GroupFor(%q) = %q, %v; want group/plain, true", "X", gid, ok)
	}
}
