package provider

import (
	"slices"
	"strings"
)

// Codex's lead picks the model of each subagent it spawns by itself, in
// spawn_agent's model argument (willz on Discord: gpt-6-astra), and its
// own config can't overrule that pick: [agents] default_subagent_model is
// only the default for a spawn that names none. settings.CodexSubagentModel
// is the model magpie puts Codex's subagents on, whatever the lead asked
// for: the gateway rewrites a subagent's request (x-openai-subagent
// collab_spawn) to it. Any model magpie serves may be set (#1633: a lead
// on a chat provider's model, its subagents wanted on that provider too).
// A subagent's task the ChatGPT backend sealed, its lead having run there,
// opens only on a ChatGPT account again: the gateway leaves such a
// subagent on the model asked for when the one set can't open it, and the
// route trace says so. "" leaves every subagent on the model its lead
// asked for.

// CodexSubagentModel is the model Codex's subagents are put on, "" for
// the one the lead asks for.
func CodexSubagentModel() string {
	return strings.TrimSpace(heldSettings().CodexSubagentModel)
}

func codexSubagentModels() []Entry {
	var chatgpt, rest []Entry
	for _, e := range catalogKept("codex").shown {
		if chatGPTEntry(e) {
			chatgpt = append(chatgpt, e)
		} else {
			rest = append(rest, e)
		}
	}
	return slices.Clip(append(chatgpt, rest...))
}

func chatGPTEntry(e Entry) bool {
	return e.Group == "" && e.Provider.Account != nil && e.Provider.Account.Agent == "codex"
}

// CodexSubagentUnserved says why Codex's subagents can't be put on id now,
// "" when magpie serves it.
func CodexSubagentUnserved(id string) string {
	served := Served()
	for _, e := range served {
		if e.ID == id {
			return ""
		}
	}
	if !CodexAccountModel(id) {
		return "magpie doesn't serve it now"
	}
	for _, e := range served {
		if chatGPTEntry(e) {
			return "no ChatGPT account on in magpie serves it"
		}
	}
	return "no ChatGPT account is on in magpie"
}

// CodexAccountModel says id names a model of a ChatGPT account's, served
// now or not: codex/<model>, or one of a provider that is a ChatGPT
// account.
func CodexAccountModel(id string) bool {
	pid, model, ok := strings.Cut(id, "/")
	if !ok || model == "" {
		return false
	}
	if pid == "codex" {
		return true
	}
	p, err := Find(pid)
	return err == nil && p.Account != nil && p.Account.Agent == "codex"
}
