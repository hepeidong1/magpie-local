package gui

// The routed models list's API: the page reads the list and saves it
// whole; the provider serves each name as a chain of targets, the
// gateway's catalog lists the names.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/yetone/magpie/internal/settings"
)

func exposedAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/exposed", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, settings.Shared().Exposed)
	})
	// save the whole list, as the page has it: a row is a custom model
	// name and what it goes to — a routing group or an entry
	mux.HandleFunc("POST /api/exposed/save", func(rw http.ResponseWriter, r *http.Request) {
		var in struct {
			Exposed []settings.Exposure `json:"exposed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			fail(rw, err)
			return
		}
		rows, err := exposedRowsSaved(in.Exposed)
		if err != nil {
			fail(rw, err)
			return
		}
		writeJSON(rw, rows)
	})
}

// exposedRowsSaved is rows as they are kept: names trimmed, unique
// without case, one word of letters, digits, dots and dashes; a chain
// carries at least one target, none of them empty. Empty-named rows are
// dropped.
func exposedRowsSaved(rows []settings.Exposure) ([]settings.Exposure, error) {
	out := make([]settings.Exposure, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		row.Name = strings.TrimSpace(row.Name)
		if row.Ref != "" && len(row.Refs) == 0 {
			// a row saved when a name mapped to one target
			row.Refs = []string{row.Ref}
		}
		row.Ref = ""
		refs := []string{}
		for _, ref := range row.Refs {
			ref = strings.TrimSpace(ref)
			if ref != "" && (len(refs) == 0 || refs[len(refs)-1] != ref) {
				refs = append(refs, ref)
			}
		}
		row.Refs = refs
		if row.Name == "" {
			continue
		}
		if strings.ContainsAny(row.Name, " \t/") || len(row.Name) > 64 {
			return nil, fmt.Errorf("a model name is one word, not %q", row.Name)
		}
		k := strings.ToLower(row.Name)
		if seen[k] {
			return nil, fmt.Errorf("the name %q is on the list twice", row.Name)
		}
		seen[k] = true
		if len(row.Refs) == 0 {
			return nil, fmt.Errorf("%q names nothing", row.Name)
		}
		out = append(out, row)
	}
	return out, settings.SetExposed(out)
}
