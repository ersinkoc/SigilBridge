package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
)

const recordVersion byte = 1

type Vault struct {
	masterKey []byte
	sessions  *repos.Sessions
	now       func() time.Time
}

func New(db *sql.DB, masterKey []byte) (*Vault, error) {
	if len(masterKey) != MasterKeySize {
		return nil, fmt.Errorf("master key must be %d bytes, got %d", MasterKeySize, len(masterKey))
	}
	keyCopy := make([]byte, len(masterKey))
	copy(keyCopy, masterKey)
	return &Vault{
		masterKey: keyCopy,
		sessions:  repos.NewSessions(db),
		now:       func() time.Time { return time.Now().UTC() },
	}, nil
}

func (v *Vault) Put(ctx context.Context, id string, plaintext []byte, metadata map[string]string) error {
	provider, err := providerFromID(id)
	if err != nil {
		return err
	}
	metadataJSON, err := encodeMetadata(metadata)
	if err != nil {
		return err
	}
	nonce, ciphertext, err := Seal(v.masterKey, plaintext, aadFor(id))
	if err != nil {
		return err
	}
	now := v.now()
	return v.sessions.Put(ctx, repos.Session{
		ID:           id,
		Provider:     provider,
		CreatedAt:    now,
		Nonce:        nonce,
		Ciphertext:   ciphertext,
		MetadataJSON: metadataJSON,
	})
}

func (v *Vault) Get(ctx context.Context, id string) ([]byte, map[string]string, error) {
	if _, err := providerFromID(id); err != nil {
		return nil, nil, err
	}
	session, err := v.sessions.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	plaintext, err := Open(v.masterKey, session.Nonce, session.Ciphertext, aadFor(session.ID))
	if err != nil {
		return nil, nil, err
	}
	metadata, err := decodeMetadata(session.MetadataJSON)
	if err != nil {
		return nil, nil, err
	}
	return plaintext, metadata, nil
}

func (v *Vault) Delete(ctx context.Context, id string) error {
	if _, err := providerFromID(id); err != nil {
		return err
	}
	return v.sessions.Delete(ctx, id)
}

func (v *Vault) List(ctx context.Context, prefix string) ([]string, error) {
	sessions, err := v.sessions.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(sessions))
	for _, session := range sessions {
		if prefix == "" || strings.HasPrefix(session.ID, prefix) {
			out = append(out, session.ID)
		}
	}
	return out, nil
}

func (v *Vault) Close() {
	if v == nil {
		return
	}
	wipe(v.masterKey)
}

func aadFor(id string) []byte {
	aad := make([]byte, 0, len(id)+1)
	aad = append(aad, id...)
	aad = append(aad, recordVersion)
	return aad
}

func providerFromID(id string) (string, error) {
	const prefix = "vault://"
	if !strings.HasPrefix(id, prefix) {
		return "", fmt.Errorf("vault id %q must start with %s", id, prefix)
	}
	rest := strings.TrimPrefix(id, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("vault id %q must follow vault://<provider>/<name>", id)
	}
	return parts[0], nil
}

func encodeMetadata(metadata map[string]string) (string, error) {
	if metadata == nil {
		metadata = map[string]string{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("encode vault metadata: %w", err)
	}
	return string(raw), nil
}

func decodeMetadata(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil, fmt.Errorf("decode vault metadata: %w", err)
	}
	if metadata == nil {
		return map[string]string{}, nil
	}
	return metadata, nil
}
