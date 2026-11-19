//go:build darwin

package update

import "golang.org/x/sys/unix"

func readOnly(path string) bool {
	var st unix.Statfs_t
	return unix.Statfs(path, &st) == nil && st.Flags&unix.MNT_RDONLY != 0
}
