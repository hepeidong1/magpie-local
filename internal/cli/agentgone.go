package cli

import (
	"os"
	"strings"
)

// magpie-local configures no agents: the CLI keeps no command that writes
// an agent's config, and these are the small stand-ins the remaining code
// needs.

// cliAgentMove is an agent a provider change "moved": never one.
type cliAgentMove struct{}

// reseatCLI ran a provider change and reseated the agents' configs; with
// no agents it just runs the change.
func reseatCLI(fn func() error) ([]cliAgentMove, error) { return nil, fn() }

// pluginAgentGone stands for the agent a plugin gives (internal/agentplug,
// gone): no plugin here adds one.
type pluginAgentGone struct {
	ID, Name, Icon string
}

// tildePath is p with the home folder as ~, for a line of output.
func tildePath(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// orNone is xs joined, or "none" when it is empty.
func orNone(xs []string) string {
	if len(xs) == 0 {
		return "none"
	}
	return strings.Join(xs, ", ")
}
