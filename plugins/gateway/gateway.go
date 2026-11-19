// Package gateway is the Routing groups page (plugins/routing draws the
// groups into it) and, besides, the keys' rows on Settings: who may call
// the gateway and what each has used, at a limit (keys.js, and the page's
// settings.js in the slots the shell gives plugin pages). The gateway
// itself is core and serves agents with the page off; while a gateway key
// is there, the page that lists and changes them stays on.
package gateway

import (
	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/pluginrt"
)

func init() {
	pluginrt.Register(pluginrt.Manifest{
		ID: "gateway", API: "^1", Requires: []string{"window", "gateway"}, Essential: true,
		Name: "Gateway page", Sub: "The routing groups, and the gateway keys' rows on Settings",
		One: "1 gateway key", Many: "{n} gateway keys",
		InUse: func() (int, error) {
			keys, err := access.List()
			return len(keys), err
		},
		Parts:  []pluginrt.Part{{Slot: "nav.page", Key: "gateway"}},
		Client: []string{"keys.js", "client.js"},
	})
}
