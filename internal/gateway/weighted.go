package gateway

// resting is shared by the callers that leave resting candidates out of
// an order the strategy code used to weigh; the weighting rounds it was
// made for (Weighted routing, group Weights) went with the strategies
// (the doc's §4.3).

// resting says whether c rests now, as restLast will find.
func resting(c candidate) bool {
	if _, ok := restOf(c.restKey()); ok {
		return true
	}
	if c.restID() != c.restKey() {
		_, ok := restOf(c.restID())
		return ok
	}
	return false
}
