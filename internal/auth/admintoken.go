package auth

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type AdminTokensFile struct {
	Tokens []AdminToken `yaml:"tokens"`
}

type AdminToken struct {
	Name    string `yaml:"name"`
	Token   string `yaml:"token"`
	Revoked bool   `yaml:"revoked"`
}

type AdminTokenStore struct {
	tokens []AdminToken
}

func LoadAdminTokens(path string) (*AdminTokenStore, error) {
	// #nosec G304 -- admin token file path is explicit local operator input.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read admin tokens %q: %w", path, err)
	}
	return ParseAdminTokens(raw)
}

func ParseAdminTokens(raw []byte) (*AdminTokenStore, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var file AdminTokensFile
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("parse admin tokens: %w", err)
	}
	if len(file.Tokens) == 0 {
		return nil, fmt.Errorf("admin token file must contain at least one token")
	}
	for _, token := range file.Tokens {
		if token.Name == "" {
			return nil, fmt.Errorf("admin token name is required")
		}
		if token.Token == "" {
			return nil, fmt.Errorf("admin token %q value is required", token.Name)
		}
	}
	return &AdminTokenStore{tokens: file.Tokens}, nil
}

func (s *AdminTokenStore) Verify(token string) (AdminToken, bool) {
	if s == nil {
		return AdminToken{}, false
	}
	for _, candidate := range s.tokens {
		if candidate.Revoked {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(candidate.Token), []byte(token)) == 1 {
			return candidate, true
		}
	}
	return AdminToken{}, false
}

func (s *AdminTokenStore) VerifyHeader(header string) (AdminToken, bool) {
	token := strings.TrimSpace(header)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return s.Verify(token)
}
