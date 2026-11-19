package sessions

import (
	"maps"
	"sync"
)

// The Sessions page answers before the files it has never read are read
// whole (hey_boyce on X: the page showed its placeholder rows for a long
// time on its first open). A first read of a long history (13 GB of Claude
// Code's and 52 GB of Codex's files on one computer) takes minutes, and
// Manage used to read all of it before it answered: the agent shown, and
// every agent with a gateway session for its ids, Codex's too while Claude
// Code was shown. It still reads at once what is cheap: a file it has a
// parse of that has only grown, read on from where the parse left off. A
// file it would read whole (none kept, an older magpie's index, a file
// written over, or a Claude Code or Codex file whose kept parse can't be
// read on after a restart) is read behind the answer, as Stats and List
// read it, and until then its row stands on what is known now: the parse
// kept of it, or a glance at its first lines (its id, folder, first prompt
// and start), with its last activity the file's time. Such a row says it
// is Pending, and the page asks again until none is.

var (
	// glances are the files glanced at, by path, while their whole read
	// is under way; each stands for the size and time it was read at
	glances = map[string]*state{}
	// overlaid are the glances put into the cache for one pass of
	// Manage, never saved with it
	overlaid = map[string]*state{}
	// behind are the paths a read behind an answer has taken, and
	// behindRuns the reads, waited for by Reset
	behind     = map[string]bool{}
	behindRuns sync.WaitGroup
)

// parsed is the parse kept of path: none for a glance put in the cache,
// which a refresh reads whole as though nothing were kept.
func parsed(path string) *state {
	s := cache[path]
	if s != nil && overlaid[path] == s {
		return nil
	}
	return s
}

// savable is the cache as it is saved: without the glances put in it.
func savable() map[string]*state {
	c := maps.Clone(cache)
	for p, g := range overlaid {
		if c[p] == g {
			delete(c, p)
		}
	}
	return c
}

// forgetBehind waits for the reads behind answers, and forgets the glances
// (Reset).
func forgetBehind() {
	behindRuns.Wait()
	mu.Lock()
	glances, overlaid, behind = map[string]*state{}, map[string]*state{}, map[string]bool{}
	mu.Unlock()
}
