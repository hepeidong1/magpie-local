package sessions

import "sort"

// A session's models are counted under the names the user gave them
// (provider.UsageAlias, `magpie model alias`): two ids of one model are one
// row. Each was priced by its own id before; the merged row adds those
// costs and is priced only when every id in it was. Everything here makes
// new slices, so what List and Stats keep for the next read is never
// changed.

// aliasModels is ms with the models alias names alike made one, the most
// used first as assembleListed sorts them.
func aliasModels(ms []Model, alias func(string) string) []Model {
	out := make([]Model, 0, len(ms))
	at := make(map[string]int, len(ms))
	for _, m := range ms {
		name := alias(m.Model)
		i, ok := at[name]
		if !ok {
			at[name] = len(out)
			m.Model = name
			out = append(out, m)
			continue
		}
		o := &out[i]
		o.Tokens.add(m.Tokens)
		o.Cost += m.Cost
		o.Priced = o.Priced && m.Priced
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Input+a.Output != b.Input+b.Output {
			return a.Input+a.Output > b.Input+b.Output
		}
		return a.Model < b.Model
	})
	return out
}

// aliasSession counts s's models under their aliases. When rows merged,
// Unpriced is the merged rows without a price, as it counted them before.
func aliasSession(s *Session, alias func(string) string) {
	if len(s.Models) == 0 {
		return
	}
	n := len(s.Models)
	s.Models = aliasModels(s.Models, alias)
	if len(s.Models) < n {
		s.Unpriced = 0
		for _, m := range s.Models {
			if !m.Priced {
				s.Unpriced++
			}
		}
	}
	s.ModelEffort = aliasEffort(s.ModelEffort, alias)
}

// aliasEffort is a session's model and effort rows with those of one
// aliased model and effort made one, in the order they first came.
func aliasEffort(es []ModelEffort, alias func(string) string) []ModelEffort {
	if len(es) == 0 {
		return es
	}
	type key struct{ model, effort string }
	out := make([]ModelEffort, 0, len(es))
	at := make(map[key]int, len(es))
	for _, e := range es {
		e.Model = alias(e.Model)
		k := key{e.Model, e.Effort}
		i, ok := at[k]
		if !ok {
			at[k] = len(out)
			out = append(out, e)
			continue
		}
		o := &out[i]
		o.Tokens.add(e.Tokens)
		o.Reasoning += e.Reasoning
		o.Cost += e.Cost
		o.Priced = o.Priced && e.Priced
	}
	return out
}
