package sessions

// A session's whole conversation, read from the agent's own file when it is
// asked for: what was said, in order, by the user, the model and its tools.
// The file is only read; magpie keeps no copy.

import ()

// Transcript is what was said in a session, in the order it was said.
type Transcript struct {
	Parts    []Part `json:"parts"`
	Cut      bool   `json:"cut,omitempty"` // there was more than is kept here
	Source   string `json:"source,omitempty"`
	Captured int    `json:"captured,omitempty"`
}

// transcriptMax is the characters of all of a transcript's parts.
const transcriptMax = 400_000

// HasTranscript says whether an agent's sessions can be read as a
// transcript.
func HasTranscript(agent string) bool {
	switch agent {
	case "claude", "qoder", "qoder-cn", "codex", "pi", "raft", "cindy", "omp", "cursor", "opencode", "kimi", "agy":
		return true
	}
	return false
}

// readable says whether a session's conversation can be read: its agent's
// can, and its file is the one they are read from (an agy conversation
// listed from its row alone has none).
func readable(s Session) bool {
	return HasTranscript(s.Agent) && !(s.Agent == "agy" && agyRowOnly(s.Path))
}

func (t *Transcript) add(_ bool, p Part) bool {
	size := 0
	for _, x := range t.Parts {
		size += len(x.Text)
	}
	if size >= transcriptMax {
		t.Cut = true
		return false
	}
	if p, ok := keep(p); ok {
		t.Parts = append(t.Parts, p)
	}
	return true
}
