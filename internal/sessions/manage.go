package sessions

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Managing sessions: every session of one agent, to look through and pick
// up again, and to delete — which never deletes. A deleted session's files
// are moved into magpie's own trash folder (<magpie dir>/trash/sessions/),
// with a note of where each came from, and a restore moves them back.
//
// Only the agents whose sessions are files of their own can be deleted:
// Claude Code's (and Qoder's and WorkBuddy's, kept the same way), Codex's,
// Pi's, omp's, Cursor CLI's (a chat's folder, its store and meta.json),
// DeepSeek Harness's (a session's folder, and its listing's cached row) and
// Droid's (a transcript and its .settings.json, and its subagents'; Droid's
// own session index is a cache it rebuilds from those files, as it does
// when it deletes an empty draft itself; zachzhao1984 on Discord) and
// Antigravity CLI's (agy: a conversation's brain folder, conversations/<id>.db
// and annotations/<id>.pbtxt, what agy's own delete removes; ysm119 on
// Discord). agy's conversation_summaries.db and jetbox_summaries_proto.pb,
// which it keeps open, keep the row, and agy lists no conversation whose
// files are gone, as a restore brings it back to its /resume list.
// Hermes's and Alma's are rows in a database the agent keeps open and
// writes, so they are only listed, and so are OpenCode's and ZCode's, and
// Cline's, whose sessions.db lists a session whose files are gone. Grok
// Build's are folders, but it copies each session's text into
// session_search.sqlite, which a delete would leave behind. Kimi Code's are
// folders, but session_index.jsonl, which Kimi writes, names each one. A
// Claude Desktop (Cowork) session is a Claude Code folder inside a session
// Desktop keeps and lists itself. The agents' indexes are left as they are: Codex's
// session_index.jsonl (names by thread id) and its state database, and
// Claude Code's history.jsonl (the prompts typed, for the up arrow), are
// written by the agent while it runs, and a name or a prompt left for a
// session that is gone does nothing.

// AgentCount is an agent with sessions on this computer, and how many.
type AgentCount struct {
	Agent     string `json:"agent"`
	Count     int    `json:"count"`
	Deletable bool   `json:"deletable"`
}

// Deletable says whether magpie can move an agent's sessions to its trash:
// those kept as files of their own, in a layout magpie knows whole.
func Deletable(agent string) bool {
	switch agent {
	case "claude", "qoder", "qoder-cn", "workbuddy", "workbuddy-ai", "codebuddy", "codex", "pi", "raft", "cindy", "omp", "cursor", "dsh", "droid", "agy":
		return true
	}
	return false
}

// ActiveWindow is how recently a session's file may have been written for
// it to count as in use, and not be deleted.
const ActiveWindow = time.Minute

// Agents are the agents with sessions on this computer, by how many.
func Agents() []AgentCount {
	storeMu.RLock()
	defer storeMu.RUnlock()
	mu.Lock()
	defer mu.Unlock()
	defer useDBs()()
	return agentCounts(allFiles())
}

func agentCounts(all []file) []AgentCount {
	n := map[string]map[string]bool{}
	// as listAgent lists them, from the parses kept: a copy of a
	// conversation is not a session of its own (ccCopies)
	copies := ccCopies(all)
	for _, f := range all {
		if copies[f.key] != "" {
			continue
		}
		if n[f.agent] == nil {
			n[f.agent] = map[string]bool{}
		}
		n[f.agent][f.key] = true
	}
	out := []AgentCount{}
	for a, keys := range n {
		out = append(out, AgentCount{Agent: a, Count: len(keys), Deletable: Deletable(a)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Agent < out[j].Agent
	})
	return out
}

// idInFile says which agents' session ids are read from inside the
// session's file, not taken from its name (as assemble does).
func idInFile(agent string) bool {
	switch agent {
	case "codex", "pi", "raft", "cindy", "omp", "dsh", "grok", "droid":
		return true
	}
	return false
}

// listed fills in what assemble leaves out of a session nothing was said
// in, as ListAgent lists it and Find finds it: how to resume it, and
// whether its conversation can be read.
func listed(s *Session) {
	if s.Resume == "" && !s.ReadOnly {
		s.Resume = resumeCommand(s.WSL, s.Agent, s.ID, s.Cwd, s.Path)
	}
	if s.Carry == nil {
		s.Carry = carries(*s)
	}
	s.Transcript = readable(*s)
}

// moved is one file or folder of a session moved to the trash: where it
// was, and its name in the trash folder's files/.
type moved struct {
	From string `json:"from"`
	Name string `json:"name"`
}

// manifest is the note kept beside a trashed session's files.
const manifest = "session.json"

// recent says whether p, or anything in it, was written within ActiveWindow.
func recent(p string, now time.Time) bool {
	hot := false
	filepath.WalkDir(p, func(q string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if fi, err := d.Info(); err == nil && now.Sub(fi.ModTime()) < ActiveWindow {
			hot = true
			return filepath.SkipAll
		}
		return nil
	})
	return hot
}
