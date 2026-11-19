package sessions

import (
	"errors"
	"io/fs"
	"os"
)

// The Codex app (ChatGPT.app's Codex, 26.x) lists conversations from a
// catalog of its own, not from the rollouts: <CODEX_HOME>/sqlite/codex.db
// (codex-dev.db for a build whose BUILD_FLAVOR isn't "prod"), table
// local_thread_catalog, a row per host and thread. It fills the catalog
// from its app-server's thread/list, but once its first full sweep is done
// (local_thread_catalog_sync_state.last_full_reconciled_at) it only takes
// in threads updated since its watermark. A row leaves the catalog only on
// the app-server's thread/deleted or thread/archived notification
// (removeThreadFromCatalog), which Codex's own delete sends. A rollout
// magpie moved to its trash was no longer listed by the app-server
// (thread/list skips a thread whose rollout is gone), but its catalog row
// stayed, so the app went on listing it (zhuddan on Discord, Codex
// 26.1023).
//
// forgetCodexCatalog does what the app does on thread/deleted: the local
// host's row of a thread whose rollout is gone is deleted and
// catalog_revision raised, in one write transaction that waits for the
// app's own. Only threads Codex's state database names with a rollout
// under that Codex folder's sessions or archived_sessions, and neither it
// nor its packed twin there, are forgotten; a rollout that can't be
// looked at counts as there. The rows of a thread in magpie's trash are
// kept in its note, and Restore puts them back, since the app wouldn't
// take in a thread that old again.

// missing says whether p is known not to be there: an error other than
// "not there" is not knowing.
func missing(p string) bool {
	_, err := os.Lstat(p)
	return errors.Is(err, fs.ErrNotExist)
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
