package sessions

// Searching what was said: the Sessions page's filter finds a session by a
// word of its conversation, not only by its title, folder, id or model
// (YOU on Discord: gateway sessions are named ses_…, and what was said is
// how they are remembered). Each session's own file is read only as far as
// its first match; a file that doesn't hold the words at all, as bytes, is
// left after one pass without decoding a line of it.

const (
	hitBefore = 36  // runes kept before a match
	hitAfter  = 120 // and after it
)

// said says whether a part is something the user or the model said, a
// match there worth more than one in a tool's output or the model's thoughts.
func said(p Part) bool {
	return p.Kind == "text" && (p.Role == "user" || p.Role == "assistant")
}

// ---- a gateway session's title ----------------------------------------------
