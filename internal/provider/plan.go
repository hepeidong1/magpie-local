package provider

import (
	"fmt"
	"strings"
)

// Kinds of plan nodes. A group's plan is the ordered list of entries it
// tries, one after another: the first that answers wins. The plan is
// stored in the shape the Routing groups page has always saved (a "race"
// node whose members are "entry" nodes) so providers.json and the page
// keep their format; ChainEntries flattens it, and the gateway tries the
// entries in that order with no racing and no cooldown — the order saved
// is the order tried.
const (
	KindEntry = "entry"
	KindRace  = "race"
	KindOrder = "order"
)

// Orderings a plan may name. The chain ignores them — the saved order is
// the order tried — but the page still reads and writes them.
const (
	OrderSpeed = "speed"
	OrderList  = "list"
)

// Defaults (doc §13), kept for the shape the editor saves.
const (
	DefaultFirstTokenMs = 75000
	DefaultTotalWaitMs  = 30000
	MaxFirstTokenMs     = 120000
	MaxTotalWaitMs      = 600000
)

// Node is one node of a group's plan tree. A "race" or "order" node's
// Members are walked in order; an "entry" node is one pinned credential
// (provider + model + credential).
type Node struct {
	Kind string `json:"kind"` // "entry" | "race" | "order"

	// entry
	Ref  string `json:"ref,omitempty"`  // "provider/model[:effort]"
	Cred string `json:"cred,omitempty"` // key id or login user; "" = the only credential
	// FirstTokenMs is this entry's first-token timeout inside a race
	// group, 0 = the default. The chain times nothing: it is saved and
	// read, never waited on.
	FirstTokenMs int `json:"firstTokenMs,omitempty"`
	// Lanes is saved and read back; the chain ignores it.
	Lanes int `json:"lanes,omitempty"`

	// race
	Members []Node `json:"members,omitempty"`
	// Seats, MaxInFlight and TotalWaitMs are saved and read back; the
	// chain ignores them (each request tries the entries one by one).
	Seats       int   `json:"seats,omitempty"`
	MaxInFlight int   `json:"maxInFlight,omitempty"`
	TotalWaitMs int   `json:"totalWaitMs,omitempty"`
	OrderBy     string `json:"orderBy,omitempty"`
	Sticky      *bool `json:"sticky,omitempty"`
}

// EntryRef names one pinned credential.
type EntryRef struct {
	Ref  string `json:"ref"`
	Cred string `json:"cred,omitempty"`
}

// Key is EntryRef as one string, "ref@cred"; cred "" gives just "ref".
func (e EntryRef) Key() string {
	if e.Cred == "" {
		return e.Ref
	}
	return e.Ref + "@" + e.Cred
}

// ParseKey splits a Key() string back into an EntryRef.
func ParseKey(k string) EntryRef {
	if i := strings.LastIndex(k, "@"); i >= 0 {
		return EntryRef{Ref: k[:i], Cred: k[i+1:]}
	}
	return EntryRef{Ref: k}
}

// ChainEntry is one step of a group's chain: the provider's model to ask
// and the credential to ask it on, in the order the group tries them.
type ChainEntry struct {
	Ref  string // "provider/model[:effort]"
	Cred string // key id or login user; "" = the only credential
}

// Key is the entry as entry-codes and the Off list spell it.
func (e ChainEntry) Key() string { return EntryRef{Ref: e.Ref, Cred: e.Cred}.Key() }

// ChainEntries is the group's ordered entries: the saved plan flattened,
// or — for a group from before the plan (Plan == nil) — one synthesized
// from its old fields. A group in the plan is walked where it stands; a
// member switched off (Off) is left out.
func (g Group) ChainEntries() []ChainEntry {
	return chainEntriesIn(g, nil)
}

// chainEntriesIn walks a group's plan (or old fields) into ordered
// entries, seen guarding a group that holds itself.
func chainEntriesIn(g Group, seen map[string]bool) []ChainEntry {
	if seen == nil {
		seen = map[string]bool{}
	}
	if seen[g.ID] {
		return nil
	}
	seen[g.ID] = true
	defer delete(seen, g.ID)
	if g.Plan != nil {
		return chainOf(g, g.Plan, seen)
	}
	return synthChain(g, seen)
}

// chainOf is a plan node's entries in order.
func chainOf(g Group, n *Node, seen map[string]bool) []ChainEntry {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case KindEntry:
		e := ChainEntry{Ref: n.Ref, Cred: n.Cred}
		if g.IsOff(e.Key()) {
			return nil // 停用的条目不进链路
		}
		return []ChainEntry{e}
	case KindRace, KindOrder:
		var out []ChainEntry
		for i := range n.Members {
			out = append(out, chainOf(g, &n.Members[i], seen)...)
		}
		return out
	}
	return nil
}

// synthChain is the chain of a group saved before the plan: its members
// in order, each expanded to one entry per credential, with the group's
// FirstToken as each entry's timeout.
func synthChain(g Group, seen map[string]bool) []ChainEntry {
	var out []ChainEntry
	for _, m := range g.Members {
		if g.IsOff(m) {
			continue
		}
		if gid, ok := strings.CutPrefix(m, GroupPrefix); ok {
			if sub, ok := groupOf(groupsIn(providerEntries()), gid); ok {
				out = append(out, chainEntriesIn(sub, seen)...)
			}
			continue
		}
		for _, e := range memberChainEntries(m) {
			out = append(out, e)
		}
	}
	return out
}

// memberChainEntries expands one old member "provider/model[:effort]" into
// one entry per credential (keys by fingerprint, subscription accounts by
// user), in the provider's own order; a provider with no credential (a
// preset that asks none) is one entry with none.
func memberChainEntries(m string) []ChainEntry {
	model, _ := MemberEffort(m)
	pid, _, ok := strings.Cut(model, "/")
	if !ok {
		return nil
	}
	var p Provider
	for _, q := range All() {
		if q.ID == pid {
			p = q
			break
		}
	}
	if p.ID == "" {
		return nil
	}
	var creds []string
	if p.Account != nil {
		if p.Account.User != "" {
			creds = append(creds, p.Account.User)
		}
		for _, q := range p.AlsoOn() {
			if q.Account != nil && q.Account.User != "" {
				creds = append(creds, q.Account.User)
			}
		}
	} else {
		for _, k := range p.KeysOn() {
			creds = append(creds, KeyID(k.Key))
		}
	}
	if len(creds) == 0 {
		creds = []string{""}
	}
	out := make([]ChainEntry, 0, len(creds))
	for _, c := range creds {
		out = append(out, ChainEntry{Ref: m, Cred: c})
	}
	return out
}

// SchedulePlan is the plan the Routing groups page edits: the saved one,
// or — for a group from before the plan — one synthesized from its old
// fields, its members expanded per credential in order.
func (g Group) SchedulePlan() *Node {
	if g.Plan != nil {
		return g.Plan
	}
	entries := g.ChainEntries()
	if len(entries) == 0 {
		return nil
	}
	root := &Node{Kind: KindRace, OrderBy: OrderList, Seats: 1, MaxInFlight: 1, TotalWaitMs: DefaultTotalWaitMs}
	for _, e := range entries {
		root.Members = append(root.Members, Node{Kind: KindEntry, Ref: e.Ref, Cred: e.Cred, Lanes: 1})
	}
	return root
}

// ValidatePlan checks a plan tree the editor sends: kinds known, a race
// has entries, the entry refs are shaped right, depths bounded, timeouts
// within range.
func ValidatePlan(n *Node) error {
	return validatePlan(n, 0)
}

func validatePlan(n *Node, depth int) error {
	if depth > 8 {
		return fmt.Errorf("调度树太深（最多 8 层）")
	}
	if n == nil {
		return fmt.Errorf("调度树缺节点")
	}
	switch n.Kind {
	case KindEntry:
		if strings.TrimSpace(n.Ref) == "" {
			return fmt.Errorf("条目缺模型引用")
		}
		if n.FirstTokenMs < 0 || n.FirstTokenMs > MaxFirstTokenMs {
			return fmt.Errorf("首字超时 %dms 超出范围 0–%d", n.FirstTokenMs, MaxFirstTokenMs)
		}
		return nil
	case KindRace, KindOrder:
		if len(n.Members) == 0 {
			return fmt.Errorf("竞速组没有条目")
		}
		if n.OrderBy != "" && n.OrderBy != OrderSpeed && n.OrderBy != OrderList {
			return fmt.Errorf("尝试顺序 %q 不认识（speed | list）", n.OrderBy)
		}
		if n.TotalWaitMs < 0 || n.TotalWaitMs > MaxTotalWaitMs {
			return fmt.Errorf("总放弃上限 %dms 超出范围 0–%d", n.TotalWaitMs, MaxTotalWaitMs)
		}
		creds := map[string]map[string]bool{}
		for i := range n.Members {
			m := &n.Members[i]
			if m.Kind == KindEntry {
				ref := strings.ToLower(m.Ref)
				set := creds[ref]
				if set == nil {
					set = map[string]bool{}
					creds[ref] = set
				}
				// one entry per credential: the same credential twice is a
				// duplicate, and a bare entry beside one of its credentials
				// would ask the model twice as well (the editor blocks both)
				if set[m.Cred] || (m.Cred != "" && set[""]) || (m.Cred == "" && len(set) > 0) {
					return fmt.Errorf("条目重复：%s 已在组里，一个条目只放一次", EntryRef{Ref: m.Ref, Cred: m.Cred}.Key())
				}
				set[m.Cred] = true
			}
			if err := validatePlan(m, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("未知的调度节点类型 %q", n.Kind)
	}
}

// savePlanGroup persists a plan group: the plan carries the group's
// whole behavior, so the old scheduling fields are cleared and only
// id/name and the plan are kept.
func savePlanGroup(g Group) error {
	g.ID = strings.ToLower(strings.TrimSpace(g.ID))
	g.Name = strings.TrimSpace(g.Name)
	if g.ID == "" {
		g.ID = GroupSlug(g.Name)
	}
	if g.ID == "" || g.ID != GroupSlug(g.ID) {
		return fmt.Errorf("a group's id must be lowercase letters, digits, dots and dashes, not %q", g.ID)
	}
	if g.Name == "" {
		g.Name = g.ID
	}
	if err := ValidatePlan(g.Plan); err != nil {
		return err
	}
	g.Members, g.Match, g.Matched, g.Placed = nil, nil, nil, nil
	g.Routing, g.Affinity, g.Sink = "", "", false
	g.FirstToken, g.Patience = 0, 0
	g.Pick, g.Rules, g.Levels = "", nil, nil // g.Off 留下：条目的启停随保存持久
	g.Classifier, g.Effort, g.Fast = "", "", nil
	g.Accounts, g.Weights = nil, nil
	g.Auto, g.Hidden, g.Disabled = false, false, false
	f, err := read()
	if err != nil {
		return err
	}
	for i := range f.Groups {
		if f.Groups[i].ID == g.ID {
			g.Disabled = f.Groups[i].Disabled && !f.Groups[i].Hidden
			f.Groups[i] = g
			return store(f)
		}
	}
	f.Groups = append(f.Groups, g)
	return store(f)
}