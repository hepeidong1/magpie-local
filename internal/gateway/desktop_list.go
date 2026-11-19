package gateway

import (
	"encoding/json"
	"errors"
	"os"
)

// A model the user picks for one of Claude Desktop's tiers is, for the
// Code tab's Claude Code, a model it doesn't know: it runs it with its
// generic prompt, not the one it has for the Claude model of that tier
// (moer2831 on Discord: 通用提示词和对应模型提示词差距挺大的). Desktop
// 4.29 and later take behavesAs, the Claude model whose prompt profile,
// capability and effort defaults apply, only from the inferenceModels list
// of a profile (models.list in its app.asar, 4.4212: z3t hands Claude Code
// modelPicker.options with each entry's behavesAs; the gateway's
// behaves_as feeds Desktop's own effort profile only). A list there
// replaces the gateway's /v1/models in Desktop's menu, and while none of
// its names is a bare tier word Desktop doesn't read /v1/models at all
// (adminListObviatesDiscovery), so the list magpie writes is the whole of
// what /v1/models gives Desktop, in its order, by its ids, each with what
// Desktop would have read off the row: its display_name as labelOverride,
// supports1m for a window of 1M and up (and prefer1m for a model listed by
// its 1M id alone), the tier of a pick. Older builds take the entries' names and
// drop the rest (2.7032's entry is a zod object, which strips unknown
// keys).

// desktopBehavesAs is the newest Claude model of each tier in Claude
// Desktop's own model catalog (v3739), which Claude Code 2.1.316 knows.
var desktopBehavesAs = map[string]string{
	"opus":   "claude-opus-5-5",
	"sonnet": "claude-sonnet-5-5",
	"haiku":  "claude-haiku-5-5",
	"fable":  "claude-fable-5-1",
}

// desktopTierOf is the tier each picked model is tagged with: a model
// picked for two, the first (desktopModels).
func desktopTierOf(picked map[string]string) map[string]string {
	tierOf := map[string]string{}
	for _, t := range DesktopTierNames {
		if id := picked[t]; id != "" && tierOf[id] == "" {
			tierOf[id] = t
		}
	}
	return tierOf
}

// desktopTiersKnown is DesktopTiers, ok false when the file is there and
// can't be read or parsed: no picks is something else than unknown picks.
func desktopTiersKnown() (map[string]string, bool) {
	out := map[string]string{}
	b, err := os.ReadFile(desktopTiersPath())
	switch {
	case errors.Is(err, os.ErrNotExist):
		return out, true
	case err != nil:
		return nil, false
	}
	if json.Unmarshal(b, &out) != nil {
		return nil, false
	}
	return out, true
}
