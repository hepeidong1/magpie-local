package provider

// 供应商没报窗口就不猜：供应商自己的清单（/v1/models、插件/账号清单）报
// 了的才算数；清单没报，或压根没抓过清单，一律 WindowUnknown（200K）。
// models.dev 按 bare id 给的窗口是别的部署的数字（orcarouter 的
// deepseek-v4.1-flash-free 曾按 opencode 的部署显示 200K，它自己是 1M），
// 既不用它兜底，Decorate 也不把清单里没写的窗口填成目录的。

import (
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/settings"
	"github.com/yetone/magpie/internal/testenv"
)

func TestListedWindowNeverGuesses(t *testing.T) {
	testenv.Sandbox(t)

	// 清单报了的：用它的
	listed := Provider{ID: "lw", Name: "LW", Preset: "own", Chat: "https://lw.test/v1", Key: "sk-lw", Models: []string{"m/one"}}
	if err := Save(listed); err != nil {
		t.Fatal(err)
	}
	if err := catalog.SaveLive("lw", "", []catalog.Model{{ID: "m/one", Context: 1000000}}); err != nil {
		t.Fatal(err)
	}
	p, err := Find("lw")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.WindowOf(catalog.Model{ID: "m/one", Context: 1000000}); got != 1000000 {
		t.Fatalf("a listed window = %d, want 1000000", got)
	}
	// 原上下文（OwnWindow）是清单里那个数，与现上下文分开给
	if got := p.OwnWindow(catalog.Model{ID: "m/one", Context: 1000000}); got != 1000000 {
		t.Fatalf("raw listed window = %d, want 1000000", got)
	}
	// 清单里这条没报窗口：200K，不拿目录给同 id 的 200K 顶上
	if got := p.WindowOf(catalog.Model{ID: "m/one"}); got != WindowUnknown {
		t.Fatalf("an unnamed window = %d, want %d", got, WindowUnknown)
	}
	// 清单没报，原上下文就是 0（页面显示 —），不是 200K
	if got := p.OwnWindow(catalog.Model{ID: "m/one"}); got != 0 {
		t.Fatalf("raw unnamed window = %d, want 0", got)
	}

	// 没抓过清单：目录给的 Context 也不算，仍 200K
	bare := Provider{ID: "nw", Name: "NW", Preset: "own", Chat: "https://nw.test/v1", Key: "sk-nw", Models: []string{"m/two"}}
	if err := Save(bare); err != nil {
		t.Fatal(err)
	}
	q, err := Find("nw")
	if err != nil {
		t.Fatal(err)
	}
	if got := q.WindowOf(catalog.Model{ID: "m/two", Context: 1048576}); got != WindowUnknown {
		t.Fatalf("a catalog window with no list = %d, want %d", got, WindowUnknown)
	}
	// 没抓过清单：原上下文也是 0（—），目录值不算
	if got := q.OwnWindow(catalog.Model{ID: "m/two", Context: 1048576}); got != 0 {
		t.Fatalf("raw catalog window = %d, want 0", got)
	}
	// 用户自己设的仍然最优先
	q.Contexts = map[string]int{"m/two": 1000000}
	if got := q.WindowOf(catalog.Model{ID: "m/two", Context: 1048576}); got != 1000000 {
		t.Fatalf("the user's window = %d, want 1000000", got)
	}
}

// 清单里没写窗口的模型，目录的同 id 窗口不会被 Decorate 填进去——填了它
// 就又成了"别的部署的数字"。
func TestDecorateLeavesWindowUnnamed(t *testing.T) {
	live := []catalog.Model{{ID: "deepseek-v4.1-flash-free"}}
	known := []catalog.Model{{ID: "deepseek-v4.1-flash-free", Context: 200000}}
	got := catalog.Decorate(live, known)
	if len(got) != 1 {
		t.Fatalf("decorated = %+v", got)
	}
	if got[0].Context != 0 {
		t.Fatalf("decorated context = %d, want none", got[0].Context)
	}
}

// 条目带着窗口是不是供应商亲口说的：清单没说时 Context 是默认的 200K，
// 但 WindowListed 为假，路由不会按这个默认值预跳过；清单说了才是真窗口。
func TestEntryTellsAssumedWindowFromListed(t *testing.T) {
	testenv.Sandbox(t)
	for _, tc := range []struct {
		id     string
		model  catalog.Model
		window int
		listed bool
	}{
		{id: "lu", model: catalog.Model{ID: "m/x"}, window: WindowUnknown, listed: false},
		{id: "ll", model: catalog.Model{ID: "m/x", Context: 262144}, window: 262144, listed: true},
	} {
		p := Provider{ID: tc.id, Name: tc.id, Preset: "own", Chat: "https://" + tc.id + ".test/v1", Key: "sk-" + tc.id, Models: []string{"m/x"}}
		if err := Save(p); err != nil {
			t.Fatal(err)
		}
		if err := catalog.SaveLive(tc.id, "", []catalog.Model{tc.model}); err != nil {
			t.Fatal(err)
		}
		saved, err := Find(tc.id)
		if err != nil {
			t.Fatal(err)
		}
		e := entryFor(*saved, tc.model, settings.Settings{})
		if e.Context != tc.window || e.WindowListed != tc.listed {
			t.Fatalf("%s: context=%d listed=%v, want %d %v", tc.id, e.Context, e.WindowListed, tc.window, tc.listed)
		}
	}
}
