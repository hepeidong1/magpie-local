package gui

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/yetone/magpie/internal/settings"
)

// Settings' gateway mode (settings.GatewayMode, "on" or "off") is kept as
// a setting, though what served by it — a page that is only the gateway
// for other machines' agents — is gone: the setting does nothing now, and
// the route keeps older pages from breaking.

// gatewayModeRoutes: POST /api/settings/gateway-mode {"mode": "on" | "off"
// | ""} sets Settings' choice.
func gatewayModeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/settings/gateway-mode", func(rw http.ResponseWriter, r *http.Request) {
		var in struct {
			Mode string `json:"mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			fail(rw, err)
			return
		}
		if in.Mode != "" && in.Mode != "on" && in.Mode != "off" {
			fail(rw, fmt.Errorf("gateway mode is on, off or \"\" (automatic), not %q", in.Mode))
			return
		}
		s := settings.Load()
		s.GatewayMode = in.Mode
		if err := settings.Save(s); err != nil {
			fail(rw, err)
			return
		}
		writeJSON(rw, settingsState())
	})
}
