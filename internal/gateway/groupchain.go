package gateway

import (
	"strings"

	"github.com/yetone/magpie/internal/provider"
)

// planChain is plan for a routing group: the group's entries, in the
// order the editor saved them, each expanded to the candidates its
// credential serves. There is no racing and no cooldown — the entries
// are tried one after another, and a failure moves to the next. The
// members passed only spell the rows' places. A member's fallbacks are
// not the group's.
func (s *Server) planChain(g provider.Group, ms []provider.Member, from provider.Protocol) ([]candidate, planned) {
	var pl planned
	out := s.chainCandidates(g, from, &pl)
	placed(ms, out)
	return out, pl
}

// chainCandidates is planChain's candidates: every entry of g's chain
// (Group.ChainEntries), in order, pinned to its credential. The rows of
// the trace are added in the same order.
func (s *Server) chainCandidates(g provider.Group, from provider.Protocol, pl *planned) []candidate {
	var out []candidate
	for _, e := range g.ChainEntries() {
		if g.IsOff(e.Key()) {
			continue // 条目的启停关着：跳过，位置留着
		}
		cs := s.entryCandidates(provider.EntryRef{Ref: e.Ref, Cred: e.Cred}, from, pl)
		for i := range cs {
			cs[i].chain = true
		}
		out = append(out, cs...)
	}
	return out
}

// entryCandidates is one entry's candidates: the provider's per-key
// planning, pinned to the entry's credential (a key by its fingerprint
// after "pid#", a subscription account by its user). A pin no credential
// answers any more — a plugin re-issues its session tokens on refresh, so
// a saved group's fingerprints go stale at once — stands for the member
// unpinned: every credential it has now serves, rather than the entry
// being dropped.
func (s *Server) entryCandidates(e provider.EntryRef, from provider.Protocol, pl *planned) []candidate {
	ref, effort := provider.MemberEffort(e.Ref)
	pid, model, ok := strings.Cut(ref, "/")
	if !ok {
		return nil
	}
	var p provider.Provider
	for _, q := range provider.All() {
		if q.ID == pid {
			p = q
			break
		}
	}
	if p.ID == "" {
		return nil
	}
	cs, aside, left, barred := perKeyEvery(p, model, from)
	_ = aside
	pl.left = append(pl.left, barredOf(barred, p, false, from, nil)...)
	for _, c := range left {
		w := weighed(c, p, weighing{}, false, from)
		w.Unlisted = true
		pl.left = append(pl.left, w)
	}
	var out []candidate
	for _, c := range cs {
		if !credMatch(c, e.Cred) {
			continue
		}
		c.effort = effort
		out = append(out, c)
	}
	if len(out) == 0 && e.Cred != "" && len(cs) > 0 {
		// the pinned credential is gone: the entry is the member, not the
		// credential, so it serves on the credentials the member has now
		for _, c := range cs {
			c.effort = effort
			out = append(out, c)
		}
	}
	for i := range out {
		out[i].no = pl.nextRow()
		pl.order = append(pl.order, weighed(out[i], p, weighing{}, false, from))
	}
	return out
}

// credMatch: the candidate is the entry's credential — a subscription
// account by its user, a key by its fingerprint. An entry pinned to no
// credential takes any of the model's.
func credMatch(c candidate, cred string) bool {
	if cred == "" {
		return true
	}
	return strings.EqualFold(c.entryCred(), cred)
}
// planGroupSeq is planGroup for the callers that ask one candidate at a
// time — the classifier's decision group (decide_group.go), a drawing
// member (draw.go), an embedding model (embeddings.go). The chain is
// already one at a time, so it is the same list; the name is kept for
// those callers to read.
func (s *Server) planGroupSeq(g provider.Group, ms []provider.Member, from provider.Protocol) ([]candidate, planned) {
	return s.planChain(g, ms, from)
}
