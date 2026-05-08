package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
)

const (
	PrefixLive = "live"
	PrefixTest = "test"
)

var tokenPattern = regexp.MustCompile(`^sb_(live|test)_[0-9a-f]{32}$`)

var (
	ErrInvalidBridgeKey = errors.New("invalid bridge key")
	ErrBridgeKeyRevoked = errors.New("bridge key revoked or not found")
	ErrScopeDenied      = errors.New("bridge key scope denied")
)

type BridgeKey struct {
	ID         string
	Hash       string
	Name       string
	Prefix     string
	CreatedAt  time.Time
	CreatedBy  string
	LastUsedAt time.Time
	RevokedAt  time.Time
	Scopes     Scopes
	Budgets    Budgets
	RateLimits RateLimits
	Metadata   map[string]string
}

type Scopes struct {
	AllowedPools  []string `json:"allowed_pools"`
	AllowedModels []string `json:"allowed_models"`
	IPAllowlist   []string `json:"ip_allowlist"`
}

type Budgets struct {
	DailyCents   int64 `json:"daily_cents"`
	MonthlyCents int64 `json:"monthly_cents"`
	HardCap      bool  `json:"hard_cap"`
}

type RateLimits struct {
	RPM int64 `json:"rpm"`
	TPM int64 `json:"tpm"`
}

type BridgeKeyStore struct {
	repo  *repos.BridgeKeys
	cache *Cache
}

func NewBridgeKeyStore(db *sql.DB, cache *Cache) *BridgeKeyStore {
	return &BridgeKeyStore{repo: repos.NewBridgeKeys(db), cache: cache}
}

func Generate(prefix string) (plaintext string, hash string, err error) {
	if prefix != PrefixLive && prefix != PrefixTest {
		return "", "", fmt.Errorf("bridge key prefix must be %q or %q", PrefixLive, PrefixTest)
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", "", fmt.Errorf("generate bridge key: %w", err)
	}
	plaintext = fmt.Sprintf("sb_%s_%s", prefix, hex.EncodeToString(random))
	return plaintext, Hash(plaintext), nil
}

func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func Prefix(token string) (string, error) {
	if !tokenPattern.MatchString(token) {
		return "", ErrInvalidBridgeKey
	}
	parts := strings.Split(token, "_")
	if len(parts) != 3 {
		return "", ErrInvalidBridgeKey
	}
	return parts[1], nil
}

func ValidateFormat(token string) error {
	_, err := Prefix(token)
	return err
}

func (s *BridgeKeyStore) Validate(ctx context.Context, token string) (*BridgeKey, error) {
	prefix, err := Prefix(token)
	if err != nil {
		return nil, err
	}
	hash := Hash(token)
	if s.cache != nil {
		if cached, ok := s.cache.Get(hash); ok {
			key := cached
			if key.Prefix != prefix {
				return nil, ErrInvalidBridgeKey
			}
			return &key, nil
		}
	}
	row, err := s.repo.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBridgeKeyRevoked
		}
		return nil, fmt.Errorf("lookup bridge key: %w", err)
	}
	key, err := fromRepoBridgeKey(row)
	if err != nil {
		return nil, err
	}
	key.Prefix = prefix
	if s.cache != nil {
		s.cache.Put(hash, key)
	}
	return &key, nil
}

func CheckScope(key *BridgeKey, pool, model, ip string) error {
	if key == nil {
		return ErrInvalidBridgeKey
	}
	if !key.RevokedAt.IsZero() {
		return ErrBridgeKeyRevoked
	}
	if !stringAllowed(key.Scopes.AllowedPools, pool) {
		return fmt.Errorf("%w: pool %q", ErrScopeDenied, pool)
	}
	if !stringAllowed(key.Scopes.AllowedModels, model) {
		return fmt.Errorf("%w: model %q", ErrScopeDenied, model)
	}
	if err := ipAllowed(key.Scopes.IPAllowlist, ip); err != nil {
		return err
	}
	return nil
}

func fromRepoBridgeKey(row repos.BridgeKey) (BridgeKey, error) {
	key := BridgeKey{
		ID:         row.ID,
		Hash:       row.Hash,
		Name:       row.Name,
		CreatedAt:  row.CreatedAt,
		CreatedBy:  row.CreatedBy,
		LastUsedAt: row.LastUsedAt,
		RevokedAt:  row.RevokedAt,
	}
	if err := decodeJSON(row.ScopesJSON, &key.Scopes); err != nil {
		return BridgeKey{}, fmt.Errorf("decode scopes for key %q: %w", row.ID, err)
	}
	if err := decodeJSON(row.BudgetsJSON, &key.Budgets); err != nil {
		return BridgeKey{}, fmt.Errorf("decode budgets for key %q: %w", row.ID, err)
	}
	if err := decodeJSON(row.RateLimitsJSON, &key.RateLimits); err != nil {
		return BridgeKey{}, fmt.Errorf("decode rate limits for key %q: %w", row.ID, err)
	}
	if err := decodeJSON(row.MetadataJSON, &key.Metadata); err != nil {
		return BridgeKey{}, fmt.Errorf("decode metadata for key %q: %w", row.ID, err)
	}
	if key.Metadata == nil {
		key.Metadata = map[string]string{}
	}
	return key, nil
}

func decodeJSON(raw string, dst any) error {
	if raw == "" {
		raw = "{}"
	}
	return json.Unmarshal([]byte(raw), dst)
}

func stringAllowed(allowed []string, value string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if candidate == value {
			return true
		}
	}
	return false
}

func ipAllowed(allowlist []string, ip string) error {
	if len(allowlist) == 0 {
		return nil
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("%w: invalid client ip %q", ErrScopeDenied, ip)
	}
	for _, raw := range allowlist {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return fmt.Errorf("invalid ip_allowlist CIDR %q: %w", raw, err)
		}
		if prefix.Contains(addr) {
			return nil
		}
	}
	return fmt.Errorf("%w: ip %q", ErrScopeDenied, ip)
}
