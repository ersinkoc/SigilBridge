package vault

import (
	"encoding/base64"
	"testing"
)

func TestLoadMasterKeyFromEnv(t *testing.T) {
	raw := make([]byte, MasterKeySize)
	for i := range raw {
		raw[i] = byte(i)
	}
	t.Setenv("SIGILBRIDGE_MASTER_KEY", base64.StdEncoding.EncodeToString(raw))

	key, err := LoadMasterKeyFromEnv("")
	if err != nil {
		t.Fatalf("LoadMasterKeyFromEnv() error = %v", err)
	}
	defer key.Wipe()
	got := key.Bytes()
	if len(got) != MasterKeySize {
		t.Fatalf("key length = %d, want %d", len(got), MasterKeySize)
	}
	got[0] = 99
	if key.Bytes()[0] == 99 {
		t.Fatalf("Bytes() returned mutable internal slice")
	}
}

func TestLoadMasterKeyFromEnvRejectsMissing(t *testing.T) {
	t.Setenv("SIGILBRIDGE_MASTER_KEY", "")
	if _, err := LoadMasterKeyFromEnv(""); err == nil {
		t.Fatal("LoadMasterKeyFromEnv() error = nil, want missing-env error")
	}
}

func TestLoadMasterKeyFromEnvRejectsBadLength(t *testing.T) {
	t.Setenv("SIGILBRIDGE_MASTER_KEY", base64.StdEncoding.EncodeToString([]byte("short")))
	if _, err := LoadMasterKeyFromEnv(""); err == nil {
		t.Fatal("LoadMasterKeyFromEnv() error = nil, want bad-length error")
	}
}

func TestMasterKeyWipe(t *testing.T) {
	raw := make([]byte, MasterKeySize)
	raw[0] = 7
	key := &MasterKey{bytes: raw}
	key.Wipe()
	for i, b := range raw {
		if b != 0 {
			t.Fatalf("raw[%d] = %d, want 0", i, b)
		}
	}
}
