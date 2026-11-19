// Package testenv gives tests a sandbox: a temp home magpie reads its
// files from, and the MAGPIE_TEST_SANDBOX marker guard tests check for.
package testenv

import (
	"os"
	"path/filepath"
	"testing"
)

// Sandbox points magpie at a fresh temp home and marks the test process.
func Sandbox(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MAGPIE_TEST_SANDBOX", "1")
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, ".local", "state"))
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "AppData", "Local"))
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))
	t.Setenv("MAGPIE_CONFIG_DIR", "")
}

// Marked is whether the process runs sandboxed.
func Marked() bool { return os.Getenv("MAGPIE_TEST_SANDBOX") == "1" }
