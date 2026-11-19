package provider

// A routed model (settings.Exposed) is one name agents ask by, mapping to
// an order of targets — routing groups ("group/<id>") or single entries
// ("provider/model[:effort]") — tried in turn: the whole of one fails
// before the next is asked. The gateway serves the name as a group of its
// own whose plan is the chain (schedule's order node), so the race
// runner, the session keeping and the winner memory all key on the name.

import (
	"strings"

	"github.com/yetone/magpie/internal/settings"
)

// ExposedChain is name as the chain group it names, or false. Real groups
// win: the group finder asks this only after its own lists came up with
// nothing. A target that names nothing now (a group removed, a model no
// provider serves) is left out of the chain; a chain of nothing is not
// served.
func ExposedChain(name string) (Group, []Member, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Group{}, nil, false
	}
	for _, row := range settings.Shared().Exposed {
		if !strings.EqualFold(row.Name, name) {
			continue
		}
		var root Node
		root.Kind = KindOrder
		var ms []Member
		for _, ref := range row.Refs {
			if gid, ok := strings.CutPrefix(ref, GroupPrefix); ok {
				tg, ok := groupByID(gid)
				if !ok {
					continue
				}
				plan := tg.Plan
				if plan == nil {
					plan = tg.SchedulePlan()
				}
				root.Members = append(root.Members, *plan)
				ms = append(ms, Member{ID: GroupPrefix + tg.ID})
				continue
			}
			root.Members = append(root.Members, Node{Kind: KindEntry, Ref: ref})
			ms = append(ms, Member{ID: ref})
		}
		if len(root.Members) == 0 {
			return Group{}, nil, false
		}
		return Group{ID: row.Name, Name: row.Name, Plan: &root}, ms, true
	}
	return Group{}, nil, false
}

// groupByID is the saved group of id, or false: the lookup a chain's
// group targets go by. Auto groups are gone and hidden ones serve no
// agent, so neither is a target.
func groupByID(id string) (Group, bool) {
	for _, g := range Groups() {
		if g.ID == id && !g.Hidden && !g.Disabled {
			return g, true
		}
	}
	return Group{}, false
}

// ExposedNames is the routed names in lower case, the set /v1/models
// hides the other groups by and lists these by.
func ExposedNames() map[string]bool {
	out := map[string]bool{}
	for _, row := range settings.Shared().Exposed {
		if row.Name != "" && len(row.Refs) > 0 {
			out[strings.ToLower(row.Name)] = true
		}
	}
	return out
}

// routedEntry is the routed name row as a catalog entry: its model is
// what the chain's first target served now answers as (the listing's
// label, window and reasoning facts read it), and it is "" when none of
// the chain's targets is served — the name is then listed by nothing.
func routedEntry(entries []Entry, all []Group, find entryOf, row settings.Exposure) Entry {
	for _, ref := range row.Refs {
		if gid, ok := strings.CutPrefix(ref, GroupPrefix); ok {
			var tg Group
			for _, g := range all {
				if g.ID == gid && !g.Hidden && !g.Disabled {
					tg = g
					break
				}
			}
			if tg.ID == "" {
				continue
			}
			ms := membersIn(entries, all, tg)
			if len(ms) == 0 || DecisionGroup(ms) {
				continue
			}
			return Entry{ID: row.Name, Model: ms[0].Model, Name: row.Name, Provider: ms[0].Provider, Group: row.Name, Named: true, Icons: []string{ms[0].Provider.Icon}}
		}
		pid, model, _ := strings.Cut(ref, "/")
		if modelft, _, fixed := strings.Cut(model, ":"); fixed {
			model = modelft
		}
		if j := find(pid, model); j >= 0 && !entries[j].Provider.Unlisted {
			e := entries[j]
			return Entry{ID: row.Name, Model: e.Model, Name: row.Name, Provider: e.Provider, Group: row.Name, Named: true, Icons: []string{e.Provider.Icon}}
		}
	}
	return Entry{}
}
