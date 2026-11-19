// Package uninstall takes magpie off this computer: `magpie uninstall` and
// Settings › Uninstall magpie.
//
// Jevik_AI on X deleted magpie's folders by hand, and Codex's config.toml
// kept openai_base_url on 127.0.0.1:3425: every Codex turn failed with a
// 502, with nothing left that knew why. Deleting magpie's files never puts
// an agent back, so uninstalling does that first, the way each agent's
// Disconnect does (what magpie changed goes back, what the user changed
// by hand since stays theirs), then removes what magpie registered with
// the system, then its own files.
//
// What it leaves: the magpie executable itself (a running program can't
// delete itself everywhere, and the user put it there), and the Library's
// folder while it holds skills or servers the user's agents use.
package uninstall

import (
	"context"
	"errors"
	"fmt"
	"github.com/yetone/magpie/internal/appdir"
	"github.com/yetone/magpie/internal/autostart"
	"github.com/yetone/magpie/internal/claudecode"
	"github.com/yetone/magpie/internal/proc"
	"github.com/yetone/magpie/internal/update"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Done is one thing uninstalling did, or couldn't.
type Done struct {
	What string `json:"what"`
	Err  string `json:"error,omitempty"`
}

// Report is what Run did.
type Report struct {
	Agents []Done `json:"agents"`
	System []Done `json:"system"`
	Data   []Done `json:"data"`
	// Kept are folders left on purpose, each with why
	Kept []Done `json:"kept,omitempty"`
	// CodexApp: a Codex app still on the config it read before, which
	// the user quits and opens
	CodexApp bool `json:"codexApp,omitempty"`
	// Exe is the magpie executable, left where it is
	Exe string `json:"exe"`
}

// Failed says whether anything couldn't be done.
func (r *Report) Failed() bool {
	for _, l := range [][]Done{r.Agents, r.System, r.Data} {
		for _, d := range l {
			if d.Err != "" {
				return true
			}
		}
	}
	return false
}

// what Run asks of the system; the tests put fakes in
var (
	autostartOff = func() error { return autostart.Set(false) }
	removeScheme = unregisterScheme
	executable   = os.Executable
)

// Agents is the first half of uninstalling, kept for callers: magpie no
// longer wires any agent, so there is nothing to disconnect and this
// records an empty list.
func Agents(ctx context.Context, r *Report) {}

// Connected are the agents magpie is in now: none.
func Connected() []struct{} { return nil }

// System removes what magpie registered with the system: its background
// service, opening at login, its Start menu shortcut and links.
func System(r *Report) {
	exe, _ := executable()
	for _, s := range []struct {
		what string
		do   func() error
	}{
		{"open at login", autostartOff},
		{"magpie:// links", func() error { return removeScheme(exe) }},
	} {
		d := Done{What: s.what}
		if err := s.do(); err != nil {
			d.Err = err.Error()
		}
		r.System = append(r.System, d)
	}
}

// Folders are magpie's own folders Data removes, and the Library's folder
// when it keeps that.
func Folders() (remove []string, keep string) {
	home, _ := os.UserHomeDir()
	add := func(p string) {
		if removable(p, home) && !slices.Contains(remove, p) {
			remove = append(remove, p)
		}
	}
	cfg := appdir.Config()
	add(cfg)
	add(appdir.Cache())
	if c, err := appdir.SystemCache(); err == nil {
		add(c)
	}
	add(appdir.WebView())
	add(claudecode.Root())
	for _, p := range webData() {
		add(p)
	}
	// a folder inside another is removed with it
	var out []string
	for _, p := range remove {
		if !slices.ContainsFunc(remove, func(q string) bool { return q != p && within(p, q) }) {
			out = append(out, p)
		}
	}
	return out, keep
}

// Data removes magpie's own files: settings, providers, sign-ins kept by
// magpie, usage, caches. With the Library in use, everything in magpie's
// folder but it goes.
func Data(r *Report) {
	remove, keep := Folders()
	for _, p := range remove {
		d := Done{What: p}
		var err error
		if keep != "" && within(keep, p) {
			err = removeAllBut(p, keep)
		} else {
			err = os.RemoveAll(p)
		}
		if err != nil {
			d.Err = err.Error()
		}
		r.Data = append(r.Data, d)
	}
	if keep != "" {
		r.Kept = append(r.Kept, Done{What: keep, Err: "skills and servers your agents use from magpie's Library"})
	}
}

// removeAllBut removes dir but keep, which is inside it.
func removeAllBut(dir, keep string) error {
	es, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range es {
		p := filepath.Join(dir, e.Name())
		switch {
		case samePath(p, keep):
		case within(keep, p):
			errs = append(errs, removeAllBut(p, keep))
		default:
			errs = append(errs, os.RemoveAll(p))
		}
	}
	return errors.Join(errs...)
}

// Run uninstalls magpie: agents, then the system, then its files. The
// app runs Data itself once it has quit (WebView2 holds its folder open
// while it runs), so it calls Agents and System only.
func Run(ctx context.Context) *Report {
	r := &Report{}
	// the service first: one still serving would put magpie back
	System(r)
	Agents(ctx, r)
	Data(r)
	r.Exe, _ = executable()
	return r
}

// AfterExit waits for process pid to end, for up to wait, then removes
// magpie's files: what the app starts as it quits.
func AfterExit(pid int, wait time.Duration) *Report {
	until := time.Now().Add(wait)
	for update.Alive(pid) && time.Now().Before(until) {
		time.Sleep(200 * time.Millisecond)
	}
	r := &Report{}
	Data(r)
	return r
}

// startAfter starts cmd outside this process, which it outlives; the
// tests put a fake in
var startAfter = func(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// DataAfterExit starts `magpie uninstall --data-after <this pid>`, which
// removes magpie's files once this process, the app, has quit: the app's
// half of Run after Agents and System.
func DataAfterExit() error {
	exe, err := executable()
	if err != nil {
		return err
	}
	return startAfter(proc.Command(exe, "uninstall", "--data-after", strconv.Itoa(os.Getpid())))
}

// removable says whether p can be one of magpie's folders: never one
// that holds more than magpie's, as the home, a root or a relative path
// would, from a variable set to one.
func removable(p, home string) bool {
	return p != "" && filepath.IsAbs(p) && filepath.Dir(p) != p && (home == "" || !within(home, p))
}

// within says whether p is dir or inside it.
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func samePath(a, b string) bool { return filepath.Clean(a) == filepath.Clean(b) }

// Says is the report as lines, for the terminal and the app's log.
func (r *Report) Says() []string {
	var out []string
	line := func(kind string, d Done) {
		if d.Err != "" {
			out = append(out, fmt.Sprintf("! %s %s: %s", kind, d.What, d.Err))
		} else {
			out = append(out, fmt.Sprintf("✓ %s %s", kind, d.What))
		}
	}
	for _, d := range r.Agents {
		line("disconnected", d)
	}
	for _, d := range r.System {
		line("removed", d)
	}
	for _, d := range r.Data {
		line("deleted", d)
	}
	for _, d := range r.Kept {
		out = append(out, "· kept "+d.What+": "+d.Err)
	}
	return out
}
