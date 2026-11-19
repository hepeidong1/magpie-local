// Package entries is the Entry combinations page: every model a provider
// serves and each key or account that can take it, one row each, grouped
// by provider, then model, then key. The row's blood bar is its cooldown
// (0 all green, 1..10 that many red, the rest grey), which the scheduler
// will read; nothing rests an entry yet, so every bar reads 0.
package entries

import "github.com/yetone/magpie/internal/pluginrt"

func init() {
	pluginrt.Register(pluginrt.Manifest{
		ID: "entries", API: "^1", Requires: []string{"window", "providers"}, Essential: true,
		Name: "Entry combinations", Sub: "Every provider model and each key or account that serves it, with its cooldown",
		One: "1 entry", Many: "{n} entries",
		Parts:  []pluginrt.Part{{Slot: "nav.page", Key: "entries"}},
		Client: []string{"client.js"},
	})
}
