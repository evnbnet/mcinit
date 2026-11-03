//go:build !windows

package server

import (
	"os"
	"syscall"
)

// sessionLocked reports whether Minecraft currently holds its lock on a
// session.lock file.
//
// This cannot be the same probe as on Windows. Java's FileChannel.lock uses
// fcntl locks, which on macOS and Linux are *advisory*: a process that does
// not ask about the lock can read and write the file as if it were free. So
// the Windows trick of reading the locked range succeeds here and would report
// a running server as stopped — which would let a backup capture a world
// mid-write, or a restore try to replace one Minecraft has open.
//
// Asking properly means attempting a conflicting lock with F_SETLK, which
// fails immediately when another process holds one.
func sessionLocked(path string) bool {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		// Missing, or not ours to write: nothing can be concluded.
		return false
	}
	defer file.Close()

	lock := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: 0, // from the start of the file
		Start:  0,
		Len:    0, // zero length means to the end, which is what Java locks
	}

	if err := syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock); err != nil {
		// EAGAIN or EACCES: somebody else holds it, which is the answer we
		// came for.
		return true
	}

	// We got the lock, so nothing was holding it. Release it again rather than
	// leaving a lock on a file that belongs to the server.
	lock.Type = syscall.F_UNLCK
	syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
	return false
}
