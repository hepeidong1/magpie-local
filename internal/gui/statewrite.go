package gui

import "net/http"

// writeStateJSON writes s to rw as writeJSON does, and returns keepState's
// sum of it (encodedSum(keptFile(s))). false when the sum wasn't found;
// s was written all the same, when it could be.
func writeStateJSON(rw http.ResponseWriter, s stateJSON) ([32]byte, bool) {
	sum, err := encodedSum(keptFile(s))
	if err != nil {
		return sum, false
	}
	writeJSON(rw, s)
	return sum, true
}
