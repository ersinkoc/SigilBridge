package vault

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
)

const (
	DefaultMasterKeyEnv = "SIGILBRIDGE_MASTER_KEY"
	MasterKeySize       = 32
)

type MasterKey struct {
	bytes  []byte
	locked bool
}

func LoadMasterKeyFromEnv(envName string) (*MasterKey, error) {
	if envName == "" {
		envName = DefaultMasterKeyEnv
	}
	raw := os.Getenv(envName)
	if raw == "" {
		return nil, fmt.Errorf("%s is required and must contain a base64-encoded 32-byte key", envName)
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", envName, err)
	}
	if len(decoded) != MasterKeySize {
		wipe(decoded)
		return nil, fmt.Errorf("%s must decode to %d bytes, got %d", envName, MasterKeySize, len(decoded))
	}

	locked, err := lockMemory(decoded)
	if err != nil {
		slog.Warn("master key mlock failed", slog.String("event", "vault_mlock_failed"), slog.String("error", err.Error()))
	}
	return &MasterKey{bytes: decoded, locked: locked}, nil
}

func (k *MasterKey) Bytes() []byte {
	if k == nil {
		return nil
	}
	out := make([]byte, len(k.bytes))
	copy(out, k.bytes)
	return out
}

func (k *MasterKey) Wipe() {
	if k == nil {
		return
	}
	if k.locked {
		_ = unlockMemory(k.bytes)
		k.locked = false
	}
	wipe(k.bytes)
}

func wipe(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
