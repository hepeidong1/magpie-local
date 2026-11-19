package gui

// agents/open opens Terminal on an agent of the Agents list: the command
// is made from the agent as detected (agentCommand), and the page sends
// only its id. A Mac-app feature; elsewhere it says so.

import (
	"encoding/json"
	"errors"
	"net/http"
)

func agentsOpenRoute(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/agents/open", func(rw http.ResponseWriter, r *http.Request) {
		var in struct{ Agent string }
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			fail(rw, err)
			return
		}
		fail(rw, errors.New("magpie-local starts no agent"))
	})
}
