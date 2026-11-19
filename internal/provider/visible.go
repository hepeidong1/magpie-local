package provider

import (
	"slices"
	"strings"
	"sync/atomic"
)

// Which of the catalog an agent is shown. Settings' Visible names, for an
// agent, the families its lists hold: a family is the tag providers and
// groups are given (magpie provider set <id> family=relay), and a
// provider's or group's id names it alone. The gateway's model list and
// what magpie writes into the agents' files both come from CatalogFor, so
// the two never differ.

// Names are what an entry answers to in a visibility: its family, and its
// provider's id or its group's id (as group/<id> too, and a found group's
// auto/<model>).
func (e Entry) Names() []string {
	var out []string
	if e.Family != "" {
		out = append(out, e.Family)
	}
	if e.Group != "" {
		out = append(out, e.Group, GroupPrefix+e.Group)
		if e.ID != GroupPrefix+e.Group { // a found group's auto/<model>
			out = append(out, e.ID)
		}
		return out
	}
	return append(out, e.Provider.ID)
}

// VisibleTo is what agent's lists are narrowed to, and whether they are.
func VisibleTo(agent string) ([]string, bool) {
	names, ok := heldSettings().Visible[strings.ToLower(agent)]
	return slices.Clone(names), ok
}

// Shows reports whether a visibility shows e.
func Shows(names []string, e Entry) bool {
	for _, n := range e.Names() {
		if slices.ContainsFunc(names, func(v string) bool { return strings.EqualFold(v, n) }) {
			return true
		}
	}
	return false
}

// Described is set by the gateway: whether an image sent to a model that
// can't see is described to it by one that can (Settings › Vision). Agents
// are then told every model takes images; told a model is text-only, they
// turn the user's image away before magpie is asked (Codex: "does not
// support image input").
var Described func() bool

// described is Described, false before the gateway sets it.
func described() bool { return Described != nil && Described() }

// CatalogFor is the catalog as agent is shown it, and what is kept from it
// (none when its lists aren't narrowed): its visibility's, less the models
// taken out of its lists one by one (HiddenModels), or, when it is shown
// only the models picked for it, those alone (PickedModels).
//
// It is kept across requests with the catalog (keptOf): a window poll
// asks it for every agent, and building each again from the whole
// catalog was ~140 MB of what twenty polls allocated. Each caller gets
// its own copy of the lists.
func CatalogFor(agent string) (shown, hidden []Entry) {
	c := catalogKept(agent)
	return slices.Clone(c.shown), slices.Clone(c.hidden)
}

// catalogKept is CatalogFor's lists as kept, only for reading.
func catalogKept(agent string) catalogLists {
	return keptOf("catalog-for\x00"+agent, func() catalogLists {
		catalogsBuilt.Add(1)
		s, h := catalogFor(agent)
		return catalogLists{slices.Clip(s), slices.Clip(h)}
	})
}

type catalogLists struct{ shown, hidden []Entry }

// catalogsBuilt counts the lists CatalogFor built, for a test.
var catalogsBuilt atomic.Int64

func catalogFor(agent string) (shown, hidden []Entry) {
	listed, hidden := ListedFor(agent)
	off := ModelOff(agent)
	for _, e := range listed {
		if off(e.ID) {
			hidden = append(hidden, e)
		} else {
			shown = append(shown, e)
		}
	}
	return shown, hidden
}

// ListedFor is the catalog agent's visibility gives it, the models it may
// pick to show or not among, and what the visibility keeps from it.
func ListedFor(agent string) (listed, kept []Entry) {
	all := catalogOf(agent)
	names, ok := VisibleTo(agent)
	if !ok {
		return InOrder(agent, all), nil
	}
	for _, e := range all {
		if Shows(names, e) {
			listed = append(listed, e)
		} else {
			kept = append(kept, e)
		}
	}
	return InOrder(agent, listed), kept
}

// HiddenModels are the ids of the entries taken out of agent's lists, a
// found group saved by its old id, group/auto-<slug>, by the one it is
// listed by now (PublicRef).
func HiddenModels(agent string) map[string]bool {
	ids := heldSettings().HiddenModels[strings.ToLower(agent)]
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[PublicRef(id)] = true
	}
	return out
}

// PickedModels are the only entries agent's lists show, by id (a found
// group by the id it is listed by now, PublicRef), and whether
// it is shown only those (#1337): a model not among them, one that came
// after the user switched to it among them, is not shown.
func PickedModels(agent string) (map[string]bool, bool) {
	ids, only := heldSettings().PickedModels[strings.ToLower(agent)]
	if !only {
		return nil, false
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[PublicRef(id)] = true
	}
	return out, true
}

// ModelOff reports whether agent's lists leave an entry out by the picks
// made one model at a time: every one not picked, when it is shown only
// the models picked for it, else the ones taken out (HiddenModels).
func ModelOff(agent string) func(id string) bool {
	if on, only := PickedModels(agent); only {
		return func(id string) bool { return !on[id] }
	}
	off := HiddenModels(agent)
	return func(id string) bool { return off[id] }
}

// sortedIDs is ids trimmed, without blanks or twins, sorted; never nil, so
// an agent shown only its picks with none picked is still saved as one.
func sortedIDs(ids []string) []string {
	keep := []string{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" && !slices.Contains(keep, id) {
			keep = append(keep, id)
		}
	}
	slices.Sort(keep)
	return keep
}

// OwnModelsHidden reports whether agent's pickers leave out the models it
// reaches on its own, offering magpie's alone (#1642).
func OwnModelsHidden(agent string) bool {
	return slices.Contains(heldSettings().OwnModelsHidden, strings.ToLower(strings.TrimSpace(agent)))
}

// Families are the families providers and groups are tagged with, sorted.
func Families() []string {
	var out []string
	add := func(f string) {
		if f != "" && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	for _, p := range All() {
		add(p.Family)
	}
	for _, g := range sharedGroups() {
		add(g.Family)
	}
	slices.Sort(out)
	return out
}

// ModelOrder is the order the user put agent's models in (OrderedModels),
// by entry id, a found group by the id it is listed by now (PublicRef);
// none when they never did.
func ModelOrder(agent string) []string {
	ids := slices.Clone(heldSettings().OrderedModels[strings.ToLower(agent)])
	for i, id := range ids {
		ids[i] = PublicRef(id)
	}
	return ids
}

// InOrder is es in agent's order: the entries ModelOrder names first, as it
// names them, then the rest as they come — a model added since goes after.
// Codex's own, a ChatGPT account's models, lead the rest, as Codex's list
// has them before magpie's.
func InOrder(agent string, es []Entry) []Entry {
	order := ModelOrder(agent)
	if len(order) == 0 {
		return es
	}
	at := make(map[string]int, len(order))
	for i, id := range order {
		at[id] = i
	}
	out := slices.Clone(es)
	rank := func(e Entry) int {
		if i, ok := at[e.ID]; ok {
			return i
		}
		if strings.EqualFold(agent, "codex") && !CodexOwn(e) {
			return len(order) + 1
		}
		return len(order)
	}
	slices.SortStableFunc(out, func(a, b Entry) int { return rank(a) - rank(b) })
	return out
}

// CodexOwn reports whether e is one of a ChatGPT account's own models,
// which the ChatGPT backend lists to Codex by its bare slug (e.Model).
func CodexOwn(e Entry) bool {
	return e.Group == "" && e.Provider.Account != nil && e.Provider.Account.Agent == "codex"
}
