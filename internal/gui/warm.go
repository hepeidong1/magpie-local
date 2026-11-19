package gui

// The app's warm-up and GC discipline, kept from the Sessions page's old
// file: the usage page's reads still take their turns with it (coldTurn
// wraps the usage endpoints), and its reads run under the tight GC.

import (
	"math"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/filememo"
	"github.com/yetone/magpie/internal/sessions"
	"github.com/yetone/magpie/internal/usage"
)

// warmSessions reads every session file once magpie is up, so the usage
// page's reads find a kept index instead of a first read of them all. Once
// a process: the app's page served to a browser (apppage.go) is a second
// Handler beside the window's.
func warmSessions() {
	if testing.Testing() {
		return
	}
	warmOnce.Do(warmStart)
}

var warmOnce sync.Once

func warmStart() {
	steadyGCStart()
	go func() {
		// a fresh install reads no agent's sessions until asked; one
		// that read them before keeps reading them (decided once)
		sessions.SettleReads()
		time.Sleep(3 * time.Second)
		sessions.Warm(func() { warmUp(warmSteps()...) })
	}()
}

// warmSteps are the reads the warm-up makes before any page asks for them:
// the usage page's whole history, then Codex's per-thread effort, which a
// first Codex page after a start otherwise read from the whole ledger while
// it waited (#2776). Both under the tight GC: the reads hold the rows they
// read.
func warmSteps() []func() {
	return []func(){
		func() { underTightGC(func() { usage.QueryPage(usage.All, usage.Filter{}, 0, 50) }) },
		func() { underTightGC(warmCodexEffort) },
	}
}

// warmCodexEffort reads what the usage page's Codex rows ask for.
var warmCodexEffort = func() { usage.CodexModelEffort(usage.All) }

// warmUp runs the warm-up's reads, then hands what they threw away back to
// the system at once. Reading a long history allocates many times what it
// keeps (a 13k-session Codex and Claude Code history: ~830 MB allocated to
// keep ~80 MB of indexes), and Go's scavenger returns those pages only over
// the next minutes: a magpie that had just started read ~470 MB for its
// first two minutes at rest.
func warmUp(steps ...func()) {
	// each file read looked at once a second, not for each session or row
	// that asks: an empty sandbox's warm-up looked at settings.json ~300
	// times on Windows (#1665); a write still reads the disk (edit.Read)
	defer filememo.Hold()()
	defer coldTurns.over.Store(true)
	for _, step := range steps {
		func() {
			coldTurns.turn <- struct{}{}
			defer coldTurnDone()
			step()
		}()
	}
	debug.FreeOSMemory()
}

// coldTurns has the warm-up's reads and the pages' first asks take turns
// until the warm-up is over: each reads after the one before it has kept
// what it read, and most then find it kept (#2278).
var coldTurns = struct {
	turn chan struct{}
	over atomic.Bool
}{turn: make(chan struct{}, 1)}

// coldTurnDone ends a turn. What the request matching kept for a next
// read goes with it: kept for scratchLinger, it sat under the next
// turn's read of the history.
func coldTurnDone() {
	usage.ReleaseMatchScratch()
	<-coldTurns.turn
}

// coldTurnMost is how long a page waits for its turn: a warm-up stuck on
// one read never keeps the pages from answering.
const coldTurnMost = 2 * time.Minute

// coldTurnsTaken is false in tests, whose pages and warm-ups are their own.
var coldTurnsTaken = !testing.Testing()

// coldTurn is h taking its turn while the warm-up isn't over; the
// warm-up's reading goes at full speed while it waits (held's
// sessions.Ask).
func coldTurn(h http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if coldTurnsTaken && !coldTurns.over.Load() {
			t := time.NewTimer(coldTurnMost)
			select {
			case coldTurns.turn <- struct{}{}:
				defer coldTurnDone()
			case <-t.C:
			}
			t.Stop()
		}
		h(rw, r)
	}
}

// underTightGC runs fn letting the heap grow by a tenth of what it holds,
// or by tightGCRoom when that is more (tightPercent), before a
// collection, not Go's default of all of it, and then puts back
// what was set; a GOGC the user set lower, or off, is kept. The usage
// page's read of the whole history holds every row of it at once, and
// Codex's effort reads the whole ledger again while the page still holds
// its rows.
//
// It sets no memory limit: it held the memory Go asked of the system to
// what it held when fn began, and a cold start then reached the limit
// with the heap all live.
func underTightGC(fn func()) {
	defer tightenGC()()
	fn()
}

// tightGC is what the reads under a tight GC hold: the first to begin sets
// it, and the last to end puts back what was set before it. Each read
// putting back what it found let two at once leave GOGC tight for good,
// when the first to begin ended before the second.
var tightGC struct {
	sync.Mutex
	n      int  // reads under it
	was    int  // GOGC before the first
	hooked bool // a retightenGC is waiting for the next collection
	steady bool // steadyGCStart has run: retightenGC sets steadyPercent between reads
}

// tightenGC begins a read under a tight GC; undo ends it.
func tightenGC() (undo func()) {
	tightGC.Lock()
	defer tightGC.Unlock()
	if tightGC.n == 0 {
		was := debug.SetGCPercent(tightGCPercent)
		if was < tightGCPercent {
			debug.SetGCPercent(was)
		}
		tightGC.was = was
		if tightGCRoom > 0 && !tightGC.hooked {
			tightGC.hooked = true
			afterNextGC(retightenGC)
		}
	}
	tightGC.n++
	return sync.OnceFunc(func() {
		tightGC.Lock()
		defer tightGC.Unlock()
		if tightGC.n--; tightGC.n == 0 {
			debug.SetGCPercent(tightGC.was)
		}
	})
}

// tightGCPercent is a tenth. Under a quarter, a first start's footprint
// peaked at 390–397 MB, and a start with its caches at 304–321 MB, on a
// live heap of ~280 MB and ~250 MB: each read, and the window's polls
// beside it, let the heap grow by ~70 MB first. Under a tenth, 350–352
// MB and 271–275 MB, for ~20% more CPU in the first start's minute and
// ~2 s a minute of polls asking ten answers every 5 s (#2278).
const tightGCPercent = 10

// tightGCRoom is the least a tight GC lets the heap grow by before a
// collection. A tenth of a small heap is a few MB: a first start's reads
// began on a ~50 MB heap and allocate ~400 MB a second, so it collected
// ~28 times a second, 2,205 times in its first 78 s, and the collections
// were 69–72 s of its ~4:10 of CPU (#2278). With this room a heap of
// over 320 MB is held to a tenth still; one below grows by 32 MB. 0 in
// tests, whose GOGC is read as tightGCPercent while a read is under it.
var tightGCRoom = func() int64 {
	if testing.Testing() {
		return 0
	}
	return 32 << 20
}()

// tightPercent is the GOGC a tight GC sets on a live heap of live bytes:
// tightGCPercent, or enough for tightGCRoom, never above was, what was
// set before (off stays off).
func tightPercent(live int64, was int) int {
	p := tightGCPercent
	if live > 0 && tightGCRoom > 0 {
		p = max(p, int(min(tightGCRoom*100/live, math.MaxInt32)))
	}
	if was < p {
		return was
	}
	return p
}

// retightenGC sets the GOGC for the heap the collection just left, while
// a read is under a tight GC, and waits for the next one.
func retightenGC() {
	tightGC.Lock()
	defer tightGC.Unlock()
	if tightGC.n == 0 && !tightGC.steady {
		tightGC.hooked = false
		return
	}
	s := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	metrics.Read(s)
	if s[0].Value.Kind() == metrics.KindUint64 {
		live := int64(s[0].Value.Uint64())
		if tightGC.n > 0 {
			debug.SetGCPercent(tightPercent(live, tightGC.was))
		} else {
			debug.SetGCPercent(steadyPercent(live))
		}
	}
	afterNextGC(retightenGC)
}

// steadyGCStart has the heap grow by half of what it holds before a
// collection, or by steadyGCRoom when that is more, not Go's default of
// all of it, for the life of the process; a GOGC the user set is kept.
// A window resting on ~60 MB live polls /api/state, /api/drift and
// /api/providers, ~10 MB thrown away a round, so at the default its
// footprint peaked at 138 MB; under half, 107-108 MB, for ~9% more CPU in
// the polls. Reads under a tight GC still set their tenth while they run.
func steadyGCStart() {
	if os.Getenv("GOGC") != "" {
		return
	}
	tightGC.Lock()
	defer tightGC.Unlock()
	tightGC.steady = true
	if tightGC.n == 0 {
		debug.SetGCPercent(steadyPercent(0))
	}
	if !tightGC.hooked {
		tightGC.hooked = true
		afterNextGC(retightenGC)
	}
}

// steadyGCPercent is the half steadyGCStart lets the heap grow by.
const steadyGCPercent = 50

// steadyGCRoom is the least it grows by: half of a small heap is a few
// MB, and a heap under 64 MB grows by 32 MB, never by more than Go's
// default.
const steadyGCRoom = 32 << 20

// steadyPercent is the GOGC steadyGCStart sets on a live heap of live
// bytes (0 before the first collection: steadyGCPercent).
func steadyPercent(live int64) int {
	p := steadyGCPercent
	if live > 0 {
		p = max(p, int(min(steadyGCRoom*100/live, 100)))
	}
	return p
}

// afterNextGC runs fn on the finalizer goroutine once the next collection
// has finished.
func afterNextGC(fn func()) {
	type tick struct{ _ *byte } // a pointer keeps it off the tiny allocator
	runtime.SetFinalizer(&tick{}, func(*tick) { go fn() })
}
