//go:build !windows

package server

import (
	"os"
	"syscall"
)

func sessionLocked(path string) bool {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {

		return false
	}
	defer file.Close()

	lock := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: 0,
		Start:  0,
		Len:    0,
	}

	if err := syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock); err != nil {

		return true
	}

	lock.Type = syscall.F_UNLCK
	syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
	return false
}
