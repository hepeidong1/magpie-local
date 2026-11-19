package sessions

import (
	"reflect"
	"time"

	"github.com/yetone/magpie/internal/catalog"
)

// keptStats are the ranges statsReadAll last read, and what they were read
// from. The Usage page asks for its range every 10 s and the Sessions page
// for 7, 30 and every day every 15 s; between two polls the files mostly
// haven't changed, and each read put every session of every range together
// again: ~1.1 GB a minute and a third, a fifth of all magpie allocated
// (#2278). Guarded by mu.
var keptStats struct {
	files  []keptFile
	today  time.Time
	gen    uint64
	ranges map[int]Stats
	// native is every listed session's agent|id, nil when no read asked
	// for it; shared, so read only
	native map[string]bool
	// the answers the reads got from the prices, Codex's names and Claude
	// Desktop's, asked again before the ranges are given out
	prices  map[string]*catalog.Price
	titles  map[string]string
	desktop map[string]string
}

// keptFile is what of a listed file a range's stats are made from: its
// parse (a published parse is never written, a changed file gets a new
// one), and what the listing says of it.
type keptFile struct {
	path, key, agent, wsl string
	size                  int64
	mod                   int64
	main, readOnly        bool
	st                    *state
}

func keptFileOf(f file) keptFile {
	return keptFile{path: f.path, key: f.key, agent: f.agent, wsl: f.m().wsl, size: f.size, mod: f.mod.UnixNano(),
		main: f.main, readOnly: f.readOnly, st: cache[f.path]}
}

// keptStill reports whether kept was taken of files as they are now, and
// prices, titles and desktop, the answers what was made from them got, are
// still what price, titles and Claude Desktop's names answer.
func keptStill(kept []keptFile, files []file, prices map[string]*catalog.Price, names, desktop map[string]string,
	price func(string) *catalog.Price, titles func(string) string) bool {
	if len(kept) != len(files) {
		return false
	}
	for i := range files {
		if kept[i] != keptFileOf(files[i]) {
			return false
		}
	}
	for model, p := range prices {
		if !reflect.DeepEqual(price(model), p) {
			return false
		}
	}
	for id, t := range names {
		if titles(id) != t {
			return false
		}
	}
	if len(desktop) > 0 {
		ids := make([]string, 0, len(desktop))
		for id := range desktop {
			ids = append(ids, id)
		}
		return reflect.DeepEqual(DesktopTitles(ids), desktopKnown(desktop))
	}
	return true
}

// desktopKnown is names without the ids that have none, as DesktopTitles
// gives them.
func desktopKnown(names map[string]string) map[string]string {
	out := map[string]string{}
	for id, n := range names {
		if n != "" {
			out[id] = n
		}
	}
	return out
}

func forgetKeptStats() {
	keptStats.files, keptStats.ranges, keptStats.native = nil, nil, nil
	keptStats.prices, keptStats.titles, keptStats.desktop = nil, nil, nil
}
