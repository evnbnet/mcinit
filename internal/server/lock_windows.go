//go:build windows

package server

import (
	"io"
	"os"
)

// sessionLocked reports whether Minecraft currently holds its lock on a
// session.lock file.
//
// Java takes a byte-range lock here. On Windows that lock is mandatory, so it
// does not prevent opening the file — only reading or writing the locked
// range. Verified against a running Paper server: the open succeeds and the
// read fails with "another process has locked a portion of the file".
//
// The probe has to be a read. A write that unexpectedly succeeded would
// corrupt the file Minecraft depends on.
func sessionLocked(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		// Missing or unreadable tells us nothing about the server.
		return false
	}
	defer file.Close()

	_, err = file.Read(make([]byte, 1))
	// io.EOF means an empty but unlocked file; only a genuine I/O error here
	// indicates the lock.
	return err != nil && err != io.EOF
}
