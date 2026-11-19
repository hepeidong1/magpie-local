package provider

import (
	"strings"
)

// Claude Desktop runs the Claude Code of its Code tab with
// CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST=1, and Claude Code then drops
// CLAUDE_CODE_SUBAGENT_MODEL, CLAUDE_CODE_SUBAGENT_MODEL_FORCE and the
// tiers' variables from the env of its settings files ("Ignoring … this
// session's provider routing is managed by the host", Claude Code
// 2.1.295). Desktop's own config has no key for it either (the env it
// builds for the Code tab, Desktop 2.31226). So the subagent model magpie
// writes into ~/.claude doesn't reach Desktop: its subagents run on the
// lead's model or on a tier's, and only the tiers could be picked.
// settings.DesktopSubagentModel is the model magpie puts every subagent
// turn of Desktop's on, whatever its lead or tier asked for: the gateway
// rewrites a subagent's request (x-claude-code-agent-id, with tools) to
// it, read on each request, so Desktop needn't restart. Its lead's turns
// and the tool-less helper calls Claude Code makes inside a subagent stay
// on the model they asked for. "" leaves every subagent as it was.

// DesktopSubagentModel is the model Desktop's subagents are put on, ""
// for the one the lead asks for. A provider renamed since it was set is
// named by its new id (RenamedRef), as a profile's model is.
func DesktopSubagentModel() string {
	id := strings.TrimSpace(heldSettings().DesktopSubagentModel)
	if id == "" {
		return ""
	}
	return RenamedRef(id)
}

// DesktopSubagentUnserved says why Desktop's subagents can't be put on
// id now, "" when magpie serves that exact model or routing group.
// Resolve isn't enough: it takes any model name under a provider that is
// on, and the subagent would go to the vendor for a model it lacks.
func DesktopSubagentUnserved(id string) string {
	id = RenamedRef(id)
	for _, e := range Served() {
		if e.ID == id {
			return ""
		}
	}
	return "magpie doesn't serve it now"
}

// DesktopSubagentEffort is the effort set for Desktop's subagent turns,
// independent of their model. Empty leaves each request's effort alone.
func DesktopSubagentEffort() string {
	return heldSettings().DesktopSubagentEffort
}
