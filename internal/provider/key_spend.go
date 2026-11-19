package provider

// A key's spend cap: how much an API key may spend, in USD, as magpie's
// own usage prices its calls (sparkling_lemur_62868 on Discord: a team's
// OpenAI key, credits granted to it, that other services spend from too,
// with nothing on OpenAI's side to stop magpie at a share of it). Once
// what magpie sent on the key in the cap's period costs the cap or more,
// routing takes the key for used up — as a usage cap holds an account —
// and goes on to the provider's other keys and its fallbacks; with none,
// the request is answered 429 with why. The key is used again when the
// period renews: the next day or month, or never for a cap in all until
// it is raised or lifted.
//
// What is counted is what the Usage page prices the key's calls at
// (usage.KeySpent), not the vendor's bill: a price magpie doesn't know
// counts nothing, and requests already under way when the cap is reached
// still finish, so the key can go a little past it.

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// SpendCap is a key's spend cap: USD over Period, which is SpendDay,
// SpendMonth or SpendTotal (in all, counted from Since).
type SpendCap struct {
	USD    float64 `json:"usd"`
	Period string  `json:"period"`
	// Since is when a cap in all was set, what it counts from; kept while
	// the cap stays one in all, its amount changed or not
	Since time.Time `json:"since,omitzero"`
}

// The periods a spend cap counts over: a calendar day or month in local
// time, or in all since it was set.
const (
	SpendDay   = "day"
	SpendMonth = "month"
	SpendTotal = "total"
)

// MaxSpendCap bounds a spend cap, in USD.
const MaxSpendCap = 1_000_000

// checkSpendCap says what is wrong with c, nil when nothing is.
func checkSpendCap(c SpendCap) error {
	switch c.Period {
	case SpendDay, SpendMonth, SpendTotal:
	default:
		return fmt.Errorf("a spend cap is per %s, per %s or in all (%s), not %q", SpendDay, SpendMonth, SpendTotal, c.Period)
	}
	if !(c.USD > 0 && c.USD <= MaxSpendCap) || math.IsNaN(c.USD) {
		return fmt.Errorf("a spend cap is an amount in USD above 0 and up to %d, not %v", MaxSpendCap, c.USD)
	}
	return nil
}

// normalKeySpendCaps is m as it is kept: key ids in lower case, caps that
// say nothing left out, a cap in all with no Since counting from the
// epoch (one written by hand); nil for none.
func normalKeySpendCaps(m map[string]SpendCap) map[string]SpendCap {
	var out map[string]SpendCap
	for k, c := range m {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" || checkSpendCap(c) != nil {
			continue
		}
		if c.Period != SpendTotal {
			c.Since = time.Time{}
		}
		if out == nil {
			out = map[string]SpendCap{}
		}
		out[k] = c
	}
	return out
}
