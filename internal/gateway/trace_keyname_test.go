package gateway

import "testing"

// TestCallerKeyName: the route keeps the gateway key's name, which the
// Requests page reads off it.
func TestCallerKeyName(t *testing.T) {
	r := Route{CallerKeyName: "李阳"}
	if got := r.CallerKeyName; got != "李阳" {
		t.Fatalf("CallerKeyName = %q, want 李阳", got)
	}
	var zero Route
	if zero.CallerKeyName != "" {
		t.Fatal("a route with no key should say \"\"")
	}
	// a route read back from the day's history (where only the JSON field
	// is) still answers the name
	if got := (Route{CallerKeyName: "贺培东"}).CallerKeyName; got != "贺培东" {
		t.Fatalf("CallerKeyName = %q, want 贺培东", got)
	}
}
