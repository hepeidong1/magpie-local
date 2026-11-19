package provider

import (
	"sync"
	"time"
)

// The entries' cooldown, in-memory (the doc's "竞速组不新增冷却": the
// write side has no caller yet — later group types add their penalty
// policies; existing quota/429/auth rests are shown from where they
// live). Keyed by entry: provider + model + credential.
type cooldownKey struct {
	provider, model, cred string
}

var (
	coolMu    sync.Mutex
	coolTimes = map[cooldownKey]int{}
	coolUntil = map[cooldownKey]time.Time{}
)

// CoolStep adds delta to the entry's cooldown count, floored at 0 and
// capped at 10 (the blood bar's cells).
func CoolStep(pid, model, cred string, delta int) {
	coolMu.Lock()
	defer coolMu.Unlock()
	k := cooldownKey{pid, model, cred}
	v := coolTimes[k] + delta
	v = min(max(v, 0), 10)
	if v == 0 {
		delete(coolTimes, k)
		return
	}
	coolTimes[k] = v
}

// Cooling is the entry's cooldown: the count (0..10) and the seconds
// left of a time-based one, if any.
func Cooling(pid, model, cred string) (times, secs int) {
	coolMu.Lock()
	defer coolMu.Unlock()
	k := cooldownKey{pid, model, cred}
	if v := coolTimes[k]; v > 0 {
		times = v
	}
	if t, ok := coolUntil[k]; ok {
		if d := time.Until(t); d > 0 {
			secs = int(d.Seconds()) + 1
		} else {
			delete(coolUntil, k)
		}
	}
	return times, secs
}

// Coolings is every entry's cooldown as one snapshot, keyed
// "provider/model@cred" (cred "" joins without the @): the pages read it
// in one call.
func Coolings() map[string][2]int {
	coolMu.Lock()
	defer coolMu.Unlock()
	out := make(map[string][2]int, len(coolTimes)+len(coolUntil))
	for k, v := range coolTimes {
		out[coolingKey(k)] = [2]int{v, 0}
	}
	for k, t := range coolUntil {
		secs := 0
		if d := time.Until(t); d > 0 {
			secs = int(d.Seconds()) + 1
		}
		if secs > 0 {
			out[coolingKey(k)] = [2]int{out[coolingKey(k)][0], secs}
		}
	}
	return out
}

// coolingKey is the stored cooldown as its "provider/model@cred" key.
func coolingKey(k cooldownKey) string {
	if k.cred == "" {
		return k.provider + "/" + k.model
	}
	return k.provider + "/" + k.model + "@" + k.cred
}
