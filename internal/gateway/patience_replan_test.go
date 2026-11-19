package gateway

// A replan's budget is the request's own — the asked group's Waits, or a
// plain provider's Patience — since a member's group Patience stopped
// being read with the group's own ordering (the doc's §4.2). These pin
// what that budget is asked of: the pause is checked against it whole,
// not member by member.

import (
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
)

func waiting(id string, ready time.Time) busyTry {
	return busyTry{c: candidate{p: provider.Provider{ID: id}}, ready: ready}
}

// no budget at all — a group whose Waits is off, a provider set
// patience=- — gives the agent the error, even with a member ready now.
func TestReplanWithoutBudgetGivesUp(t *testing.T) {
	now := time.Now()
	list := []busyTry{waiting("a", now), waiting("b", now)}
	if _, _, ok := replan(list, 0, now, 0); ok {
		t.Fatal("replan asked a member again with no budget to wait on")
	}
	if _, _, ok := replan(list, replanRounds, now, time.Minute); ok {
		t.Fatal("replan asked a member again past its rounds")
	}
}

// the soonest Retry-After beyond the budget is the whole round's end: no
// member's own setting keeps the request waiting past it, so the agent
// has the error rather than a pause the budget can't cover.
func TestReplanPausesPastTheBudgetGiveUp(t *testing.T) {
	now := time.Now()
	list := []busyTry{waiting("a", now.Add(3*time.Minute))}
	if _, _, ok := replan(list, 0, now, 30*time.Second); ok {
		t.Fatal("replan kept a pause the budget doesn't cover")
	}
	// the same round with a budget that does cover it asks the member
	next, wait, ok := replan(list, 0, now, 5*time.Minute)
	if !ok {
		t.Fatal("replan gave up on a pause the budget covers")
	}
	// replan 内部重读钟，等待值比这里记的 3 分钟少几十微秒是常态：
	// 按本文件自己的整秒约定容一秒
	if wait < 3*time.Minute-time.Second {
		t.Fatalf("wait = %v, want at least the Retry-After", wait)
	}
	if len(next) != 1 || next[0].p.ID != "a" {
		t.Fatalf("next = %v, want the one member", next)
	}
}

// a vendor's time is read in whole seconds: one back within a second past
// the budget is asked as the budget ends, not given up on (zoecats). A
// lone member shows the clamp alone: without it its Retry-After is the
// round's wait, which lands past the budget and ends the request.
func TestReplanClampsASecondPastTheBudget(t *testing.T) {
	now := time.Now()
	end := now.Add(time.Minute)
	next, _, ok := replan([]busyTry{waiting("a", end.Add(500*time.Millisecond))}, 0, now, time.Minute)
	if !ok {
		t.Fatal("replan gave up on a member a hair past its budget")
	}
	if len(next) != 1 || next[0].p.ID != "a" {
		t.Fatalf("next = %v, want the one member asked as the budget ends", next)
	}
	// with a second member back at once the clamp doesn't move the pause:
	// the one in a moment goes first, the clamped one follows
	list := []busyTry{waiting("a", now), waiting("b", end.Add(500*time.Millisecond))}
	next, wait, ok := replan(list, 0, now, time.Minute)
	if !ok {
		t.Fatal("replan gave up on a round the budget covers")
	}
	if len(next) != 2 || next[0].p.ID != "a" || next[1].p.ID != "b" {
		t.Fatalf("next = %v, want a then b", next)
	}
	if wait > 5*time.Second {
		t.Fatalf("wait = %v, want the round's own pause, not the clamped one's", wait)
	}
}
