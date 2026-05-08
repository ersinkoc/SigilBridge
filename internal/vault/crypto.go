package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const NonceSize = 12

func Seal(masterKey, plaintext, aad []byte) ([]byte, []byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generate AES-GCM nonce: %w", err)
	}
	ciphertext, err := sealWithNonce(masterKey, nonce, plaintext, aad)
	if err != nil {
		return nil, nil, err
	}
	return nonce, ciphertext, nil
}

func Open(masterKey, nonce, ciphertext, aad []byte) ([]byte, error) {
	aead, err := newAEAD(masterKey)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("AES-GCM nonce must be %d bytes, got %d", aead.NonceSize(), len(nonce))
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("open AES-GCM ciphertext: %w", err)
	}
	return plaintext, nil
}

func sealWithNonce(masterKey, nonce, plaintext, aad []byte) ([]byte, error) {
	aead, err := newAEAD(masterKey)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("AES-GCM nonce must be %d bytes, got %d", aead.NonceSize(), len(nonce))
	}
	return aead.Seal(nil, nonce, plaintext, aad), nil
}

func newAEAD(masterKey []byte) (cipher.AEAD, error) {
	if len(masterKey) != MasterKeySize {
		return nil, fmt.Errorf("master key must be %d bytes, got %d", MasterKeySize, len(masterKey))
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	return aead, nil
}
