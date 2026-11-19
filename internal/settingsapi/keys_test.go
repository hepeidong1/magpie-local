package settingsapi

import (
	"slices"
	"testing"

	"github.com/yetone/magpie/internal/settings"
)

// TestKeysAreWhatTheSaveWrites is the guard the package doc promises: a key
// in Keys must name a field of settings.Settings by its JSON name — one
// that doesn't makes `magpie settings list` panic on a zero reflect.Type,
// and `magpie settings set` accept a setting that can never take. Fields
// that left Settings (the fork's masking surface went in one commit) left
// Keys here the same day; this test keeps the two lists from drifting
// apart again.
func TestKeysAreWhatTheSaveWrites(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Keys {
		if seen[k] {
			t.Fatalf("%q is in Keys twice", k)
		}
		seen[k] = true
		f, ok := Field(k)
		if !ok {
			t.Fatalf("Keys names %q, but settings.Settings has no such json field; remove it from Keys", k)
		}
		if f.Type == nil {
			t.Fatalf("Field(%q) came back without a type", k)
		}
		if _, ok := Value(settings.Settings{}, k); !ok {
			t.Fatalf("Value cannot read %q off a Settings", k)
		}
	}
	// Choices must name Keys too: a Choices entry whose key left Keys is
	// dead weight, and one Key misses its Choices entry names no set of
	// values in `magpie settings list`.
	for k := range Choices {
		if !seen[k] {
			t.Fatalf("Choices names %q, which is not in Keys", k)
		}
	}
	for _, k := range []string{"theme", "lang", "tray", "currency", "trayUsageEvery", "searchFirst"} {
		if _, ok := Choices[k]; !ok {
			t.Fatalf("Choices lost %q: magpie settings list would show no set of values for it", k)
		}
	}
	// the fork pins the language to Chinese: Langs stays a one-slot list
	if !slices.Equal(settings.Langs, []string{"zh"}) {
		t.Fatalf("settings.Langs = %v, want [zh] alone", settings.Langs)
	}
}
