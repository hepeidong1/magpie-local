package provider

// Weights a provider's accounts once carried (AccountWeights, #841) are
// no longer read: the scheduling lives in a routing group's plan. The
// field stays on the saved provider so nothing is lost, and what a save
// loads is kept normalized by normalAccountWeights, as it always was.

// normalAccountWeights is m as it is kept: keys in lower case, weights
// within 2–MaxKeyWeight, the default 1 left out; nil for none.
func normalAccountWeights(m map[string]int) map[string]int {
	var out map[string]int
	for u, w := range m {
		u = accountKey(u)
		if u == "" || w <= 1 {
			continue
		}
		if out == nil {
			out = map[string]int{}
		}
		out[u] = min(w, MaxKeyWeight)
	}
	return out
}
