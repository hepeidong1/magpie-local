package settingsapi

import (
	"encoding/json"
	"testing"

	"github.com/yetone/magpie/internal/settings"
)

// A save from the settings page carries only its own choices; everything
// else must survive as cur has it.
func TestMergeKeepsRoutedModelsAndModelPrefs(t *testing.T) {
	cur := settings.Settings{
		Theme:  "dark",
		Lang:   "zh",
		Exposed: []settings.Exposure{{Name: "Smart", Refs: []string{"group/a", "group/b"}}},
		ModelNames:  map[string]string{"p/m": "Mine"},
	}
	body, _ := json.Marshal(map[string]any{"theme": "light", "lang": "en"})
	in, err := Merge(body, cur, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Exposed) != 1 || in.Exposed[0].Name != "Smart" {
		t.Errorf("Exposed lost: %+v", in.Exposed)
	}
	if in.ModelNames["p/m"] != "Mine" {
		t.Errorf("ModelNames lost: %+v", in.ModelNames)
	}
	if in.Theme != "light" || in.Lang != "en" {
		t.Errorf("the page's own choices not applied: %q %q", in.Theme, in.Lang)
	}
}
