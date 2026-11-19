package sessions

import ()

// Codex lists, in its history and its resume picker, only the sessions made
// with the provider it uses now: a rollout's session_meta names its
// model_provider, and so does the thread's row in Codex's state database
// (state_<n>.sqlite). Sessions made through CC Switch ("custom") are gone
// from that list once Codex uses magpie, though their files are all there
// (#887). MoveCodexProvider changes a session's provider, in its files and
// in that row, and nothing else: its messages, id, title and archive stay as
// they were. A copy of each file as it was is kept in magpie's trash folder
// first, and moving it back to the provider it had is the undo.

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
