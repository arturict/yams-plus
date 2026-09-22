//go:build !windows

package backup

import "syscall"

// openNoFollow stops a restore from writing through a symlink that is already
// sitting at the destination. Restore runs as root, so following one would let
// an entry under the state directory redirect a root-owned write.
const openNoFollow = syscall.O_NOFOLLOW
