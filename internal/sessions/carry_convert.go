package sessions

// Carrying a conversation over to an agent that can't read the session's
// own file (0xPlato on X): Claude Code's is converted into a new Codex
// session, and Codex's into a new Claude Code session, each in that agent's
// own folder and form, so its own resume opens it. The source file is only
// read.
//
// What goes over: what the user and the agent said, a shell command and what
// it printed as the target's own tool call and result (Claude Code's Bash,
// Codex's exec_command), every other tool's call and result as words the
// agent said (the target has no tool of that name), and the user's pasted
// images. What doesn't: reasoning. Claude Code's thinking is checked by its
// signature and Codex's by its encrypted_content, neither of which the other
// vendor can read, and a summary of it is only what the model thought. The
// target picks its model as it does for any resumed session: Codex from its
// config.toml (the rollout names none and keeps no base_instructions, so
// Codex uses its own), Claude Code from its settings; the assistant lines
// say "<synthetic>", Claude Code's own name for a line no model wrote here,
// so magpie's usage counts none of them as a call.
//
// The conversion runs when the user asks for it (CarryOver), never on a
// listing, and the one made from a file's bytes is handed back again while
// it is there, so a second click doesn't make a second session.

import (
	"bytes"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/yetone/magpie/internal/proc"
)

// convertsTo are the agents a session of an agent can be converted for.
var convertsTo = map[string]string{"claude": "codex", "codex": "claude"}

// agentThere says whether an agent is installed: its command, or its folder.
// A var so tests can say. Answers are kept a little while, as the listing
// asks it of every session on every read.
var agentThere = func(agent string) bool {
	thereMu.Lock()
	defer thereMu.Unlock()
	if t, ok := thereAt[agent]; ok && time.Since(t.at) < 30*time.Second {
		return t.ok
	}
	ok := false
	switch agent {
	case "claude":
		ok = lookOK("claude") || dirOK(ClaudeDir())
	case "codex":
		ok = lookOK("codex") || dirOK(CodexDir())
	}
	thereAt[agent] = there{ok, time.Now()}
	return ok
}

type there struct {
	ok bool
	at time.Time
}

var (
	thereMu sync.Mutex
	thereAt = map[string]there{}
)

func lookOK(name string) bool { _, err := proc.Look(name); return err == nil }
func dirOK(dir string) bool   { st, err := os.Stat(dir); return err == nil && st.IsDir() }

// ---- reading Claude Code

// ---- reading Codex

// ---- writing

// ord is a JSON object whose fields are written in the order given, as the
// agent's own files have them (some readers look for a line's type near
// its start).
type ord []kv

type kv struct {
	k string
	v any
}

// o is an ord of key, value pairs.
func o(pairs ...any) ord {
	out := make(ord, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, kv{pairs[i].(string), pairs[i+1]})
	}
	return out
}

func (m ord) MarshalJSON() ([]byte, error) {
	b := []byte{'{'}
	for i, f := range m {
		if i > 0 {
			b = append(b, ',')
		}
		k, _ := marshalPlain(f.k)
		v, err := marshalPlain(f.v)
		if err != nil {
			return nil, err
		}
		b = append(append(append(b, k...), ':'), v...)
	}
	return append(b, '}'), nil
}

func marshalPlain(v any) ([]byte, error) {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
