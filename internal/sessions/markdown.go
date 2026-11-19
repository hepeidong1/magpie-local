package sessions

// A session's whole conversation as a Markdown file, to keep or to read
// elsewhere (#1276): what the Sessions page's Show conversation shows, read
// again from the agent's own file without the page's cuts.

import (
	"time"
)

func when(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05 -07:00")
}
