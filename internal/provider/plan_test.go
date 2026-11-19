package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/testenv"
)

func putP(t *testing.T, p Provider) {
	t.Helper()
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
}

func TestChainEntriesMigratesOrderGroup(t *testing.T) {
	testenv.Sandbox(t)
	putP(t, Provider{ID: "p1", Name: "P1", Chat: "https://p1.test/v1", Key: "sk-1"})
	putP(t, Provider{ID: "p2", Name: "P2", Chat: "https://p2.test/v1", Key: "sk-2"})
	if err := SaveGroup(Group{ID: "g", Name: "G", Members: []string{"p1/m1", "p2/m2"}}); err != nil {
		t.Fatal(err)
	}
	var g Group
	for _, x := range Groups() {
		if x.ID == "g" {
			g = x
		}
	}
	entries := g.ChainEntries()
	if len(entries) != 2 {
		t.Fatalf("entries = %+v, want the two members", entries)
	}
	if entries[0].Ref != "p1/m1" || entries[1].Ref != "p2/m2" {
		t.Fatalf("entries = %+v, order not the members'", entries)
	}
	if entries[0].Cred == "" {
		t.Fatal("entry0 has no credential pinned")
	}
}

// TestMigratedGroupPlanIsAFlatChain: an old group (no Plan) is read as a
// flat chain — the plan the page edits is a race in list order whose
// entries are the members, and ChainEntries is that same order. The
// chain times nothing: the old FirstToken is not carried onto entries.
func TestMigratedGroupPlanIsAFlatChain(t *testing.T) {
	testenv.Sandbox(t)
	putP(t, Provider{ID: "p1", Name: "P1", Chat: "https://p1.test/v1", Key: "sk-1"})
	g := Group{ID: "g1", Name: "G1", Members: []string{"p1/m1"}, FirstToken: 12}
	root := g.SchedulePlan()
	if root == nil || root.Kind != KindRace {
		t.Fatalf("root kind = %q, want race (the shape the page edits)", root.Kind)
	}
	if root.OrderBy != OrderList {
		t.Fatalf("orderBy = %q, want list", root.OrderBy)
	}
	if len(root.Members) != 1 || root.Members[0].Kind != KindEntry || root.Members[0].Ref != "p1/m1" {
		t.Fatalf("members = %+v", root.Members)
	}
	if root.Members[0].FirstTokenMs != 0 {
		t.Fatalf("entry firstTokenMs = %d, want 0 (the chain times nothing)", root.Members[0].FirstTokenMs)
	}
	entries := g.ChainEntries()
	if len(entries) != 1 || entries[0].Ref != "p1/m1" {
		t.Fatalf("chain = %+v", entries)
	}
}

func TestSavePlanGroupRoundTrip(t *testing.T) {
	testenv.Sandbox(t)
	plan := &Node{
		Kind: KindRace, Seats: 1, MaxInFlight: 1, TotalWaitMs: 45000, OrderBy: OrderList,
		Members: []Node{
			{Kind: KindEntry, Ref: "p1/m1", Cred: "abc", FirstTokenMs: 8000},
			{Kind: KindEntry, Ref: "p2/m2"},
		},
	}
	if err := SaveGroup(Group{ID: "rg", Name: "RG", Plan: plan}); err != nil {
		t.Fatal(err)
	}
	var g Group
	for _, x := range Groups() {
		if x.ID == "rg" {
			g = x
		}
	}
	if g.Plan == nil || g.Plan.Kind != KindRace {
		t.Fatalf("plan = %+v", g.Plan)
	}
	// a plan saves its own fields, and clears the old ones
	if g.Plan.Seats != 1 || g.Plan.MaxInFlight != 1 {
		t.Fatalf("seats/max = %d/%d, want 1/1", g.Plan.Seats, g.Plan.MaxInFlight)
	}
	if len(g.Members) != 0 || g.Routing != "" || g.FirstToken != 0 {
		t.Fatalf("old fields not cleared: %+v", g)
	}
	if g.Plan.Members[0].FirstTokenMs != 8000 || g.Plan.Members[0].Cred != "abc" {
		t.Fatalf("entry0 = %+v", g.Plan.Members[0])
	}
	// the saved plan's chain is the entries in order
	entries := g.ChainEntries()
	if len(entries) != 2 || entries[0].Ref != "p1/m1" || entries[1].Ref != "p2/m2" {
		t.Fatalf("chain = %+v", entries)
	}
}

// TestChainEntriesMigratesEveryStrategy: the old routing strategies and
// manual all become the same thing — the old routing, affinity, sink,
// pick and weights are dropped, and the members expand to per-credential
// entries in the members' own order.
func TestChainEntriesMigratesEveryStrategy(t *testing.T) {
	testenv.Sandbox(t)
	putP(t, Provider{ID: "p1", Name: "P1", Chat: "https://p1.test/v1", Key: "sk-1"})
	putP(t, Provider{ID: "p2", Name: "P2", Chat: "https://p2.test/v1", Key: "sk-2"})
	for i, routing := range []string{"", Rotate, Weighted, Manual} {
		g := Group{ID: fmt.Sprintf("g-%d", i), Name: "G " + routing, Members: []string{"p1/m1", "p2/m2"}, Routing: routing, Sink: routing == Rotate}
		if routing == Manual {
			g.Pick = "p1/m1"
		}
		if err := SaveGroup(g); err != nil {
			t.Fatal(err)
		}
		var saved Group
		for _, x := range Groups() {
			if x.ID == g.ID {
				saved = x
			}
		}
		entries := saved.ChainEntries()
		if len(entries) != 2 || entries[0].Ref != "p1/m1" || entries[1].Ref != "p2/m2" {
			t.Fatalf("%s: entries = %+v", routing, entries)
		}
		if entries[0].Cred == "" {
			t.Fatalf("%s: entry0 = %+v", routing, entries[0])
		}
	}
}

func TestSavePlanGroupValidation(t *testing.T) {
	testenv.Sandbox(t)
	cases := []struct {
		name string
		plan *Node
		want string
	}{
		{"race empty", &Node{Kind: KindRace, Members: nil}, "没有条目"},
		{"order empty", &Node{Kind: KindOrder}, "没有条目"},
		{"cap below seats", &Node{Kind: KindRace, Seats: 4, MaxInFlight: 2, Members: []Node{{Kind: KindEntry, Ref: "p/m"}}}, ""}, // saved, read back; seats don't gate
		{"unknown kind", &Node{Kind: "weighed", Members: []Node{{Kind: KindEntry, Ref: "p/m"}}}, "未知的调度节点类型"},
		{"firstToken range", &Node{Kind: KindRace, Seats: 1, TotalWaitMs: 30000, Members: []Node{{Kind: KindEntry, Ref: "p/m", FirstTokenMs: 999999}}}, "超出范围"},
		{"entry ref empty", &Node{Kind: KindEntry}, "条目缺模型引用"},
		{"orderBy unknown", &Node{Kind: KindRace, Seats: 1, OrderBy: "fastest", Members: []Node{{Kind: KindEntry, Ref: "p/m"}}}, "尝试顺序"},
		{"orderBy list accepted", &Node{Kind: KindRace, Seats: 1, OrderBy: OrderList, Members: []Node{{Kind: KindEntry, Ref: "p/m"}}}, ""},
	}
	for _, c := range cases {
		err := SaveGroup(Group{ID: "v" + strings.ReplaceAll(c.name, " ", "-"), Name: "V", Plan: c.plan})
		if c.want == "" {
			if err != nil {
				t.Errorf("%s: err = %v, want it saved", c.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}

// TestMigratedChainWalksAGroupInsideAGroup: an old group inside an old
// group keeps its inner members in the flattened chain, in order.
func TestMigratedChainWalksAGroupInsideAGroup(t *testing.T) {
	testenv.Sandbox(t)
	putP(t, Provider{ID: "p1", Name: "P1", Chat: "https://p1.test/v1", Key: "sk-1"})
	putP(t, Provider{ID: "p2", Name: "P2", Chat: "https://p2.test/v1", Key: "sk-2"})
	if err := SaveGroup(Group{ID: "inner", Name: "Inner", Members: []string{"p2/m2"}}); err != nil {
		t.Fatal(err)
	}
	if err := SaveGroup(Group{ID: "outer", Name: "Outer", Members: []string{"p1/m1", "group/inner"}}); err != nil {
		t.Fatal(err)
	}
	var outer Group
	for _, x := range Groups() {
		if x.ID == "outer" {
			outer = x
		}
	}
	entries := outer.ChainEntries()
	if len(entries) != 2 {
		t.Fatalf("chain = %+v, want both the outer member and the inner group's", entries)
	}
	if entries[0].Ref != "p1/m1" || entries[1].Ref != "p2/m2" {
		t.Fatalf("inner group dropped or out of order: %+v", entries)
	}
}

// 一个凭据的条目只放一次：完全相同的重复，和裸条目 beside 它的某个
// 凭据条目（会把同一个模型问两遍），保存都被拒；不同凭据是不同条目，
// 不受影响。
func TestValidatePlanRejectsDuplicateEntries(t *testing.T) {
	race := func(members ...Node) *Node {
		return &Node{Kind: KindRace, Seats: 1, Members: members}
	}
	entry := func(ref, cred string) Node {
		return Node{Kind: KindEntry, Ref: ref, Cred: cred}
	}
	for name, members := range map[string][]Node{
		"完全相同":     {entry("a/m1", "k1"), entry("a/m1", "k1")},
		"裸的带上凭据的":  {entry("a/m1", ""), entry("a/m1", "k1")},
		"带凭据的带上裸的": {entry("a/m1", "k1"), entry("a/m1", "")},
	} {
		if err := ValidatePlan(race(members...)); err == nil {
			t.Errorf("%s: 应当被拒", name)
		} else if !strings.Contains(err.Error(), "条目重复") {
			t.Errorf("%s: 报错不对：%v", name, err)
		}
	}
	if err := ValidatePlan(race(entry("a/m1", "k1"), entry("a/m1", "k2"))); err != nil {
		t.Errorf("不同凭据是不同条目：%v", err)
	}
}