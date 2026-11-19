package gateway

import (
	"strings"

	"github.com/yetone/magpie/internal/provider"
)

// Active 结构补充：请求当前所在 try 的应答模型与该条目的 6 位识别码，
// 请求页与路由组的条目板直接拿它对行（识别码就是为此设的同步键）。

func activeEntryCode(rt *Route, t *Try) string {
	id := t.ID
	key := ""
	if i := strings.Index(id, "#"); i >= 0 {
		key = id[i+1:]
	} else if i := strings.Index(id, "@"); i >= 0 {
		key = id[i+1:]
	} else if p, err := provider.Find(rt.Provider); err == nil {
		if ks := p.KeysOn(); len(ks) == 1 {
			key = provider.KeyID(ks[0].Key)
		}
	}
	if key == "" {
		return ""
	}
	// 条目键的两种账号形态都认：登录列表里的 login:user，和供应商只
	// 有一个登录时的 account:user；码表按原样大小写存，查找折叠大小写
	model := rt.Provider + "/" + t.Model + "@"
	if code := provider.EntryCodeFind(model + "login:" + key); code != "" {
		return code
	}
	if code := provider.EntryCodeFind(model + "account:" + key); code != "" {
		return code
	}
	if code := provider.EntryCodeFind(model + key); code != "" {
		return code
	}
	return provider.EntryCode(model + "login:" + key)
}
