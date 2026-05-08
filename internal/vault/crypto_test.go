package vault

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func TestOpenKnownAESGCMVector(t *testing.T) {
	key := make([]byte, MasterKeySize)
	nonce := make([]byte, NonceSize)
	ciphertext, err := hex.DecodeString("530f8afbc74536b9a963b4f1c4cb738b")
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := Open(key, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if len(plaintext) != 0 {
		t.Fatalf("plaintext len = %d, want 0", len(plaintext))
	}
}

func TestSealWithNonceKnownAESGCMVector(t *testing.T) {
	key := make([]byte, MasterKeySize)
	nonce := make([]byte, NonceSize)
	ciphertext, err := sealWithNonce(key, nonce, nil, nil)
	if err != nil {
		t.Fatalf("sealWithNonce() error = %v", err)
	}
	if got := hex.EncodeToString(ciphertext); got != "530f8afbc74536b9a963b4f1c4cb738b" {
		t.Fatalf("ciphertext = %s, want known vector", got)
	}
}

func TestSealOpenRandomRoundTrip(t *testing.T) {
	key := make([]byte, MasterKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	for i := range 1000 {
		plaintext := make([]byte, i%257)
		aad := []byte{byte(i), byte(i >> 8)}
		if _, err := rand.Read(plaintext); err != nil {
			t.Fatal(err)
		}
		nonce, ciphertext, err := Seal(key, plaintext, aad)
		if err != nil {
			t.Fatalf("Seal() error = %v", err)
		}
		got, err := Open(key, nonce, ciphertext, aad)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if !bytes.Equal(got, plaintext) {
			t.Fatalf("round trip mismatch at iteration %d", i)
		}
	}
}

func TestOpenRejectsTampering(t *testing.T) {
	key := make([]byte, MasterKeySize)
	nonce, ciphertext, err := Seal(key, []byte("secret"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext[0] ^= 0xff
	if _, err := Open(key, nonce, ciphertext, []byte("aad")); err == nil {
		t.Fatal("Open() error = nil, want tamper failure")
	}
}

func TestOpenRejectsAADMismatch(t *testing.T) {
	key := make([]byte, MasterKeySize)
	nonce, ciphertext, err := Seal(key, []byte("secret"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(key, nonce, ciphertext, []byte("different")); err == nil {
		t.Fatal("Open() error = nil, want AAD mismatch failure")
	}
}
