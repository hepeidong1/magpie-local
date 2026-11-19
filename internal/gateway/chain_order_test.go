package gateway

// A plan group's candidate order is the plan's own: the chain tried in
// turn, the whole of one failing before the next is asked (the doc's
// §4.2). The old keeping moved the conversation's group to the front of
// the candidate list, and a chain whose conversation had answered on a
// later group skipped every group the user put before it — without
// asking them at all. The conversation's stick still leads its own
// group's race (schedule.moveSticky, the group's own switch); a
// provider's own keys keep their keeping, their order being no plan of
// the user's.

import (
	"net/http"
	"testing"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

func TestChainKeepsItsGroupsInOrder(t *testing.T) {
	testenv.Sandbox(t)
	forgetSpeeds()
	defer forgetSpeeds()
	clearSticks()
	s, p := planThreeKeys(t, "")
	cs, pl := s.plan(p, "p9/m1", provider.Chat)
	if len(cs) != 3 {
		t.Fatalf("candidates = %d, want three", len(cs))
	}
	in := http.Header{}
	body := []byte(`{"model":"one","messages":[{"role":"user","content":"hi"}]}`)
	// a conversation the second key answered, with a cache worth keeping
	answered(stickKey("group/one", in, body), cs[1], 0, Usage{Input: 4096})
	got, _, aff, _ := affine("group/one", "", false, in, provider.Chat, body, body, cs, pl)
	if !aff.Kept {
		t.Fatalf("the keeping's verdict was lost: why %q", aff.Why)
	}
	if got[0].p.Key != cs[0].p.Key {
		t.Fatalf("a group's candidates were reordered: first %q, want the plan's own %q", got[0].p.Key, cs[0].p.Key)
	}
	// a provider's own keys keep the conversation: the second key leads
	clearSticks()
	answered(stickKey(p.ID+"/m1", in, body), cs[1], 0, Usage{Input: 4096})
	got, _, aff, _ = affine(p.ID+"/m1", "", false, in, provider.Chat, body, body, cs, pl)
	if !aff.Kept {
		t.Fatalf("a provider's keeping was lost: why %q", aff.Why)
	}
	if got[0].p.Key != cs[1].p.Key {
		t.Fatalf("a provider's conversation was not kept: first %q, want the answered %q", got[0].p.Key, cs[1].p.Key)
	}
}
