//go:build !linux

package vault

func lockMemory(_ []byte) (bool, error) {
	return false, nil
}

func unlockMemory(_ []byte) error {
	return nil
}
