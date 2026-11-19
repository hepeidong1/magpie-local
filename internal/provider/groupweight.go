package provider

import (
	"fmt"
	"strings"
)

// renamedWeights is weights with each member as to spells it now, those
// it gives "" dropped; nil when none is left.
func renamedWeights(weights map[string]int, to func(string) string) map[string]int {
	var out map[string]int
	for m, w := range weights {
		if m = to(m); m == "" {
			continue
		}
		if out == nil {
			out = map[string]int{}
		}
		out[m] = max(out[m], w)
	}
	return out
}

// cleanWeights keeps g.Weights to its members as Members spells them,
// each weighing 2 to MaxKeyWeight, as a provider's keys do (#841): 1, the
// default, and 0 are left out, and one outside the range is refused.
func cleanWeights(entries []Entry, g *Group) error {
	var err error
	g.Weights = renamedWeights(g.Weights, func(m string) string {
		id, _ := memberFastIn(entries, strings.TrimSpace(m))
		id = cleanMember(entries, id)
		w := g.Weights[m]
		if w < 0 || w > MaxKeyWeight {
			err = fmt.Errorf("%s weighs %d: a member's weight is 1 to %d", id, w, MaxKeyWeight)
		}
		for _, have := range g.Members {
			if have == id && w > 1 {
				return id
			}
		}
		return ""
	})
	return err
}
