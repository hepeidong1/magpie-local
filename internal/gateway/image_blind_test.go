package gateway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/yetone/magpie/internal/appdir"
	"github.com/yetone/magpie/internal/testenv"
)

// TestBlindSurvivesRestart:拒图学到的"看不见"记录跨重启活着。原先是
// 纯内存（Server.blind），models.dev 对小中转没有 images 数据，"清单
// 没说"的条目每次重启都要再白跑一趟才重新学会。
func TestBlindSurvivesRestart(t *testing.T) {
	testenv.Sandbox(t)
	s := New()
	s.refuseSight("p1", "m1")
	s.refuseSight("p1", "m1") // 已知的不再重写
	b, err := os.ReadFile(filepath.Join(appdir.Config(), "image-blind.json"))
	if err != nil {
		t.Fatalf("the refusal was not saved: %v", err)
	}
	var keys []string
	if err := json.Unmarshal(b, &keys); err != nil {
		t.Fatalf("the saved file is not a key list: %v", err)
	}
	if !slices.Contains(keys, "p1/m1") {
		t.Fatalf("image-blind.json = %v, want it to hold p1/m1", keys)
	}
	// 新的 Server 装回：拒过的模型还"看不见"
	s2 := New()
	s2.loadBlind()
	if !s2.sightRefused("p1", "m1") {
		t.Fatal("a loaded server does not remember the refusal")
	}
	if s2.sightRefused("p2", "m2") {
		t.Fatal("an unlearned model counts blind")
	}
}
