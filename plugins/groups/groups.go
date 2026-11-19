// Package groups is routing groups configuration UI support for magpie v3.
// The groups' configuration (list/edit/save) is kept, but user groups are
// not served into the gateway routing logic in this build.
package groups

import (
	"github.com/yetone/magpie/internal/pluginrt"
	"github.com/yetone/magpie/internal/provider"
)

func init() {
	pluginrt.Register(pluginrt.Manifest{
		ID: "groups", API: "^1", Requires: []string{"window", "providers"},
		Name: "Routing groups", Sub: "Routing groups configuration (routing logic disabled in this build)",
		One: "1 group in use", Many: "{n} groups in use",
		InUse: provider.NamedGroups,
		Parts: []pluginrt.Part{{Slot: "part", Key: "groups"}},
		Apply: func(c *pluginrt.Ctx) error {
			// Intentionally do not serve user groups into gateway routing.
			return nil
		},
	})
}
