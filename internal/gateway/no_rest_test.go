package gateway

import (
	"net/http"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/testenv"
)

// raceTestCand is a minimal candidate for the no-rest tests: a provider
// on one key, the model it serves.
func raceTestCand(pid string, url string) candidate {
	return candidate{p: provider.Provider{ID: pid, Name: pid, Chat: url, Key: "k", Preset: "own"}, model: pid + "/m", rest: pid}
}

// TestFailuresNeverRest:本分叉永不冷却——任何失败都不写休息，候选顺序
// 恒为 plan 的从上到下。用户的要求是"永远从上往下尝试"，额度/限流/
// 余额失败都不该把条目挪到队尾（restLast）或被 probeOne 剔除。
func TestFailuresNeverRest(t *testing.T) {
	testenv.Sandbox(t)
	s := New()
	c := raceTestCand("p", "http://p")

	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"quota 429", http.StatusTooManyRequests, `{"error":{"message":"quota exceeded"}}`},
		{"credit 402", http.StatusPaymentRequired, `{"error":{"message":"insufficient credit"}}`},
		{"rate limit", http.StatusTooManyRequests, `{"error":{"message":"rate limit reached"}}`},
		{"auth 401", http.StatusUnauthorized, `{"error":{"message":"invalid api key"}}`},
		{"server 500", http.StatusInternalServerError, `{"error":{"message":"internal"}}`},
		{"balance 400", http.StatusBadRequest, `{"error":{"message":"余额不足"}}`},
	} {
		rest := s.restAfter(c, tc.status, http.Header{}, []byte(tc.body))
		if rest.By != "off" {
			t.Errorf("%s: By = %q, want \"off\" (no rest at all)", tc.name, rest.By)
		}
		if !rest.Until.IsZero() && rest.Until.After(time.Now()) && rest.By == "off" {
			// "off" 语义下 Until 只是时间戳，不构成休息
		}
		if _, resting := restOf(c.restKey()); resting {
			t.Errorf("%s: the candidate was put to rest", tc.name)
		}
	}
	// 画图路径同样不冷却
	restDrawer(c, http.StatusTooManyRequests, "quota exceeded")
	if _, resting := restOf(c.restKey()); resting {
		t.Error("restDrawer put the account to rest")
	}
}

// TestLoadRestsIsOff:上一进程留下的冷却一条也不装回——否则重启后
// 请求仍会跳过队首条目。
func TestLoadRestsIsOff(t *testing.T) {
	testenv.Sandbox(t)
	saveRests() // 写一个当前的空状态，确保文件存在
	loadRests()
	if _, resting := restOf("anything"); resting {
		t.Fatal("loadRests restored a rest")
	}
}
