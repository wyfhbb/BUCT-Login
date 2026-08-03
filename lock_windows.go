//go:build windows

package main

import (
	"syscall"
)

// errSharingViolation is what Windows returns when the file is already held by
// someone who opened it without sharing
const errSharingViolation = syscall.Errno(32)

// acquireLock takes an exclusive lock on path without waiting.
//
// A share mode of 0 means no other process may open the file at all while this
// handle lives, and Windows closes the handle when the process ends, so a
// crashed run cannot leave the lock behind.
func acquireLock(path string) (func(), bool, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, false, err
	}

	handle, err := syscall.CreateFile(
		name,
		syscall.GENERIC_WRITE,
		0, // no sharing
		nil,
		syscall.OPEN_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		if err == errSharingViolation {
			return nil, false, nil
		}
		return nil, false, err
	}

	return func() { syscall.CloseHandle(handle) }, true, nil
}
