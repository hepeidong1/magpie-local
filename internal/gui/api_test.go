package gui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/testenv"
)

// §4.4: magpie finds groups on its own no more, so the found and skip
// actions are gone: the Routing page still calls them until its own
// task, and hits the unknown-action default, a 404.
func TestGroupsFoundAndSkipActionsGone(t *testing.T) {
	testenv.Sandbox(t)
	mux := http.NewServeMux()
	groupRoutes(mux)
	for _, action := range []string{"found", "skip"} {
		r := httptest.NewRequest("POST", "/api/groups/"+action, strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusNotFound {
			t.Fatalf("POST /api/groups/%s: %d, want 404", action, w.Code)
		}
	}
}
