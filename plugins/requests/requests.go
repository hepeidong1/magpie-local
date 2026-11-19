// Package requests is the Requests page: the requests out at the vendors
// right now, one per row (the build's own concurrency ceiling aside, the
// table grows with the traffic and holds at least three rows), and under
// them the log of the calls done, newest first.
package requests

import "github.com/yetone/magpie/internal/pluginrt"

func init() {
	pluginrt.Register(pluginrt.Manifest{
		ID: "requests", API: "^1", Requires: []string{"window"},
		Name: "Requests", Sub: "The requests out at the vendors now, and the log of the ones done",
		Parts:  []pluginrt.Part{{Slot: "nav.page", Key: "requests"}},
		Client: []string{"client.js", "requests.css"},
	})
}
