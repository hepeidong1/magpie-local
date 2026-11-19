package provider

import "testing"

// TestGroupNameBeatsSameNamedModel:组名与请求字面相等（忽略大小写）时
// 走组，即使某个供应商直供同名模型。上游规则"同名模型优先直连"会让
// 用户建的组永远进不去——请求被当成直连发给恰好也叫这个名字的供应商，
// 组内排在前面的条目全被跳过。
func TestGroupNameBeatsSameNamedModel(t *testing.T) {
	all := []Group{
		{ID: "g1", Name: "GLM-5.3-flash"},
	}
	for _, asked := range []string{"GLM-5.3-flash", "glm-5.3-flash", " GLM-5.3-Flash "} {
		if got, ok := groupNamed(all, asked); !ok || got != "group/g1" {
			t.Errorf("groupNamed(%q) = %q, %v; want group/g1 — a group's own name is its own", asked, got, ok)
		}
	}
	// 两个组同名的仍不猜
	two := []Group{{ID: "a", Name: "X"}, {ID: "b", Name: "X"}}
	if got, ok := groupNamed(two, "X"); ok {
		t.Errorf("two groups named X: picked %q", got)
	}
	// 没有这个名的组时什么也不认
	if got, ok := groupNamed(all, "other"); ok {
		t.Errorf("groupNamed(other) = %q, want none", got)
	}
}
