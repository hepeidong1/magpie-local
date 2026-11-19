package gui

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/yetone/magpie/internal/appdir"
)

// magpie-local configures no agents: the GUI's agent-shaped API is either
// gone or answers with nothing. These are the small stand-ins the code
// paths that remained need.

// agentMove is what a provider change "moved" agents to: none, ever.
type agentMove struct{}

// reseat ran a provider change and reseated affected agents' configs;
// with no agents to reseat it just runs the change.
func reseat(fn func() error) ([]agentMove, error) {
	return nil, fn()
}

// errNoAgent is what agentNamed answers: magpie-local configures no agents.
var errNoAgent = errors.New("magpie-local configures no agents")

// isDir says whether p is a folder.
func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// downloads is the folder a saved file goes to: the system's Downloads,
// or the home folder when it can't be found.
func downloads() string {
	home, _ := os.UserHomeDir()
	for _, d := range []string{appdir.Downloads(), filepath.Join(home, "Downloads")} {
		if d != "" && isDir(d) {
			return d
		}
	}
	return home
}
