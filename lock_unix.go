//go:build !windows

package main

import (
	"os"
	"syscall"
)

// acquireLock takes an exclusive lock on path without waiting.
//
// The lock lives on the open file description, so the kernel drops it when the
// process ends however it ends: a crashed or killed run cannot leave a stale
// lock behind, which a lock built out of "create the file, delete it at exit"
// would.
func acquireLock(path string) (func(), bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, false, err
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, false, nil
		}
		return nil, false, err
	}

	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, true, nil
}
