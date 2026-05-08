//go:build linux

package vault

import "golang.org/x/sys/unix"

func lockMemory(data []byte) (bool, error) {
	if len(data) == 0 {
		return false, nil
	}
	if err := unix.Mlock(data); err != nil {
		return false, err
	}
	return true, nil
}

func unlockMemory(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return unix.Munlock(data)
}
