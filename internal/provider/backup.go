package provider

// A backup carries providers.json's entries as they are stored, and puts
// them back on another machine (see internal/backup).

import (
	"slices"

	"github.com/yetone/magpie/internal/plugin"
)

// handOver is HandOver: a var so a test can see it asked.
var handOver = HandOver

// Stored is the providers as providers.json keeps them: keys included,
// signed-in accounts only as the model picks the user made for them. A
// file that can't be read is an error, never none: a backup or a sync
// carrying none would take every provider away where it is put back.
func Stored() ([]Provider, error) {
	f, err := read()
	return f.Providers, err
}

// Carried is the plugins a backup or a sync carries: those added from npm
// or a git repository, on or off as the user left them, with their options:
// without keys, those whose names look like a key are left out, as a
// provider's secret headers are. A plugin added from a file or folder is
// this computer's alone, and a plugin's sign-ins (plugin-auth.json) stay
// here, as a subscription's do.
func Carried(keys bool) []plugin.Entry {
	var out []plugin.Entry
	for _, e := range plugin.Load().Plugins {
		if plugin.IsPath(e.Spec) {
			continue
		}
		if !keys {
			e.Options = withoutSecretOptions(e.Options)
		}
		out = append(out, e)
	}
	return out
}

// withoutSecretOptions is opts without the values whose names look like a
// key, at any depth: nil when nothing is left.
func withoutSecretOptions(opts map[string]any) map[string]any {
	var strip func(v any) any
	strip = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, v := range x {
				if !SecretHeader(k) {
					out[k] = strip(v)
				}
			}
			return out
		case []any:
			out := make([]any, len(x))
			for i, v := range x {
				out[i] = strip(v)
			}
			return out
		}
		return v
	}
	out, _ := strip(map[string]any(opts)).(map[string]any)
	if len(out) == 0 {
		return nil
	}
	return out
}

// NotSignedIn is the subscriptions among ps (as a backup or a sync brings
// them: their model picks and settings alone) that no account here is
// signed in to: they are listed once one is, and are to be signed in to on
// this computer. A subscription the user removed from magpie is not one.
func NotSignedIn(ps []Provider) []string {
	have := map[string]bool{}
	for _, p := range All() {
		have[p.ID] = true
	}
	return notSignedIn(ps, have)
}

// notSignedIn is NotSignedIn with the ids of the providers here given.
func notSignedIn(ps []Provider, have map[string]bool) []string {
	var out []string
	for _, p := range ps {
		// the same subscription signed in here under its other id (a
		// built-in moved onto its plugin on one computer and not on the
		// other, #1940) is signed in
		if p.ID == "" || !OnlyPicks(p) || p.Hidden || have[p.ID] || have[samePlace(p.ID)] || slices.Contains(out, p.ID) {
			continue
		}
		out = append(out, p.ID)
	}
	return out
}

// OnlyPicks is whether p, as providers.json keeps it, is a subscription's
// model picks and settings alone, with no endpoint or key of its own: it
// is signed in to, never given a key.
func OnlyPicks(p Provider) bool { return !hasEndpoint(normalize(p)) }
