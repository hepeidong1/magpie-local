package provider

// 每个条目（供应商/模型@密钥或账号）的 6 位识别码：随机、互不重复、
// 跨重启稳定，供应商列表的条目与它一一对应，路由组之后按它区分组内
// 条目。识别码首次见到条目时生成，存在 entry-codes.json（providers.json
// 旁），随条目增删自然增长。

import (
	"crypto/rand"
	"log"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yetone/magpie/internal/appdir"
)

// entryCodeLen is how long an entry's code is: six characters.
const entryCodeLen = 6

// entryCodeAlphabet leaves out 0/O and 1/I/L: a code read off a row or
// typed into a group can't lose its way on lookalikes.
const entryCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

var (
	entryCodesMu   sync.Mutex
	entryCodesRead bool
	entryCodes     map[string]string
)

func entryCodesFile() string { return appdir.Join(appdir.Config(), "entry-codes.json") }

func loadEntryCodes() {
	if entryCodesRead {
		return
	}
	entryCodesRead = true
	b, err := os.ReadFile(entryCodesFile())
	if err != nil {
		return
	}
	if err := json.Unmarshal(b, &entryCodes); err != nil {
		log.Printf("entry-codes: the store doesn't parse (%v); codes mint again and differ from the saved ones", err)
	}
}

// EntryCodeFind is the entry's code when the entry is already known, ""
// when it isn't: a lookup that never mints, exact first, then over the
// store case-insensitively.
func EntryCodeFind(key string) string {
	if key == "" {
		return ""
	}
	entryCodesMu.Lock()
	defer entryCodesMu.Unlock()
	loadEntryCodes()
	return findEntryCode(key)
}

// saveEntryCodes writes the store, through a file renamed over the old one
// at 0o600 like every config file's. A write that fails is logged, never
// silent: codes that didn't land are minted again after a restart, and
// "stable across a restart" quietly breaking is worth a line in the log.
func saveEntryCodes() {
	b, err := json.MarshalIndent(entryCodes, "", "  ")
	if err != nil {
		log.Printf("entry-codes: %v", err)
		return
	}
	p := entryCodesFile()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		log.Printf("entry-codes: %v", err)
		return
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		log.Printf("entry-codes: %v", err)
		return
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		log.Printf("entry-codes: %v", err)
	}
}

// EntryCode is the entry's own code, made once and kept for good: a fresh
// random code the first time the entry is seen, the same one after.
// "" for an empty key. The lookup folds case: the gateway's slot keys
// lowercase the account user, the store keeps the case the Provider list
// minted with — one entry, one code, whichever form asks. Mint and store
// are one locked span with the lookup re-done inside: two callers that
// arrive together for one entry both see it missing, and only one code
// would win the store, leaving the loser's minted code dangling.
func EntryCode(key string) string {
	if key == "" {
		return ""
	}
	entryCodesMu.Lock()
	defer entryCodesMu.Unlock()
	loadEntryCodes()
	if c := findEntryCode(key); c != "" {
		return c
	}
	used := map[string]bool{}
	for _, c := range entryCodes {
		used[c] = true
	}
	var c string
	for {
		b := make([]byte, entryCodeLen)
		if _, err := rand.Read(b); err != nil {
			return ""
		}
		c = ""
		for _, x := range b {
			c += string(entryCodeAlphabet[int(x)%len(entryCodeAlphabet)])
		}
		if !used[c] {
			break
		}
	}
	if entryCodes == nil {
		entryCodes = map[string]string{}
	}
	entryCodes[key] = c
	saveEntryCodes()
	return c
}

// findEntryCode is the store's lookup, case-folded, for callers already
// holding entryCodesMu.
func findEntryCode(key string) string {
	if c, ok := entryCodes[key]; ok && len(c) == entryCodeLen {
		return c
	}
	lk := strings.ToLower(key)
	for k, c := range entryCodes {
		if strings.ToLower(k) == lk && len(c) == entryCodeLen {
			return c
		}
	}
	return ""
}
