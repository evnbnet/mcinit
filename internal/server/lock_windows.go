//go:build windows

package server

import (
	"io"
	"os"
)

func sessionLocked(path string) bool {
	file, err := os.Open(path)
	if err != nil {

		return false
	}
	defer file.Close()

	_, err = file.Read(make([]byte, 1))

	return err != nil && err != io.EOF
}
