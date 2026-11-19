package gateway

import (
	"math"
	"strings"
	"sync"
	"time"

	"github.com/yetone/magpie/internal/usage"
)

// Speed lately, of each plan entry by its key ("provider/model@cred",
// lowercased): a decayed average of its streamed answers' time to first
// token and of how fast they wrote, beside the last answer's own
// numbers. Kept in memory, a few numbers each, written once as an answer
// ends (noteSpeed) — the race orders its entries by it (orderBy speed)
// and the group editor shows it; the usage history is never read for it.
// A restart forgets them.

// speedHalfLife: a sample counts half after this, against those since.
// speedStale: this long after its last answer an entry counts as not
// measured lately, and the race gives it a turn now and then to be
// measured again, so one that was slow once isn't kept last for good.
const (
	speedHalfLife = 20 * time.Minute
	speedStale    = time.Hour
)

// speedMost bounds the records: past it, those not measured lately go.
const speedMost = 4096

type speedRec struct {
	ttft, ttftW float64   // the average ms to the first token, and its samples' weight
	tps, tpsW   float64   // the average output tokens a second, and its samples' weight
	at, tpsAt   time.Time // the last sample of each
	// what one answer said, not an average: the group editor shows these
	// beside each entry ("上次首字 / 上次速度").
	lTTFT   int64
	lTTFTAt time.Time
	lTPS    float64
	lTPSAt  time.Time
}

var speeds = struct {
	sync.Mutex
	// e: each entry by its plan key ("provider/model@cred", lowercased),
	// the same view the scheduler orders a race by and the group editor
	// reads.
	e map[string]speedRec
}{e: map[string]speedRec{}}

func decayed(w float64, since time.Duration) float64 {
	if since <= 0 {
		return w
	}
	return w * math.Exp2(-since.Seconds()/speedHalfLife.Seconds())
}

// fold is avg of weight w, decayed by since, with x added at weight 1.
func fold(avg, w float64, since time.Duration, x float64) (float64, float64) {
	w = decayed(w, since)
	return (avg*w + x) / (w + 1), w + 1
}

// noteSpeed records an answer of c's: ttft ms from its try's start to its
// first content, and, when its pace was told, tokens written in w ms
// after it (usage.DecodeOf). A reply that wasn't streamed has no ttft and
// tells nothing.
func noteSpeed(c candidate, ttft int64, tokens int, w int64, now time.Time) {
	if ttft <= 0 {
		return
	}
	speeds.Lock()
	defer speeds.Unlock()
	sample := float64(0)
	if tokens > 0 && w > 0 {
		sample = float64(tokens) * 1000 / float64(w)
	}
	note := func(k string, by map[string]speedRec) {
		r := by[k]
		r.ttft, r.ttftW = fold(r.ttft, r.ttftW, now.Sub(r.at), float64(ttft))
		r.at = now
		r.lTTFT, r.lTTFTAt = ttft, now
		if sample > 0 {
			r.tps, r.tpsW = fold(r.tps, r.tpsW, now.Sub(r.tpsAt), sample)
			r.tpsAt = now
			r.lTPS, r.lTPSAt = sample, now
		}
		if _, had := by[k]; !had && len(by) >= speedMost {
			for kk, o := range by {
				if now.Sub(o.at) > speedStale {
					delete(by, kk)
				}
			}
		}
		by[k] = r
	}
	note(strings.ToLower(c.entryKey()), speeds.e)
}

// entrySpeed is one entry's decayed average as the race orders by it:
// the ms to its first token and the tokens a second after it, 0 for what
// nothing recent tells of. key is the plan's entry key, lowercased.
func entrySpeed(key string, now time.Time) (ttft, tps float64) {
	speeds.Lock()
	r, ok := speeds.e[key]
	speeds.Unlock()
	if !ok || now.Sub(r.at) > speedStale {
		return 0, 0
	}
	if !r.tpsAt.IsZero() && now.Sub(r.tpsAt) <= speedStale {
		tps = r.tps
	}
	return r.ttft, tps
}

// EntrySpeed is what one entry answered lately, as the group editor
// shows it: the last answer's own numbers, and the average the race
// orders its entries by.
type EntrySpeed struct {
	LastTTFT int64   `json:"lastTtft,omitempty"` // ms to the first token, its last answer
	LastTPS  float64 `json:"lastTps,omitempty"`  // output tokens a second, its last answer
	TTFT     float64 `json:"ttft,omitempty"`     // the same, averaged lately
	TPS      float64 `json:"tps,omitempty"`
	At       int64   `json:"at,omitempty"` // unix ms of that last answer
}

// EntrySpeeds is every entry measured lately, by its plan key
// ("provider/model@cred", lowercased; the cred left off when the
// provider has no credential of its own).
func EntrySpeeds() map[string]EntrySpeed {
	now := time.Now()
	speeds.Lock()
	defer speeds.Unlock()
	out := make(map[string]EntrySpeed, len(speeds.e))
	for k, r := range speeds.e {
		if now.Sub(r.at) > speedStale {
			continue
		}
		e := EntrySpeed{LastTTFT: r.lTTFT, TTFT: r.ttft, At: r.lTTFTAt.UnixMilli()}
		if !r.tpsAt.IsZero() && now.Sub(r.tpsAt) <= speedStale {
			e.TPS = r.tps
			if !r.lTPSAt.IsZero() {
				e.LastTPS = r.lTPS
			}
		}
		out[k] = e
	}
	return out
}

// noteTrySpeed is noteSpeed of a try that answered, its pace as
// usage.DecodeOf counts it for the Requests and Usage pages.
func noteTrySpeed(c candidate, t Try, out, reasoning int, now time.Time) {
	tokens, w := 0, int64(0)
	if !t.ReasoningHidden || t.FirstText > 0 {
		tokens, w = usage.DecodeOf(out, reasoning, t.Millis, t.TTFT, t.FirstText, t.Flow, t.FirstReasoning)
	}
	noteSpeed(c, t.TTFT, tokens, w, now)
}

// forgetSpeeds drops every speed: a test starts anew.
func forgetSpeeds() {
	speeds.Lock()
	speeds.e = map[string]speedRec{}
	speeds.Unlock()
}
