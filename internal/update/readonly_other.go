//go:build !darwin

package update

// readOnly is false off the Mac: only a Mac app can be translocated.

func readOnly(string) bool { return false }
