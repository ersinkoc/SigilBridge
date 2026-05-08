package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"
)

const (
	AdminSessionCookieName = "sigilbridge_admin"
	AdminSessionTTL        = 15 * time.Minute
)

var ErrInvalidSession = errors.New("invalid admin session")

type AdminSessionManager struct {
	signingKey []byte
	now        func() time.Time
	ttl        time.Duration
}

type adminClaims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func NewAdminSessionManager(masterKey []byte) (*AdminSessionManager, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(masterKey))
	}
	key, err := deriveAdminSigningKey(masterKey)
	if err != nil {
		return nil, err
	}
	return &AdminSessionManager{
		signingKey: key,
		now:        time.Now,
		ttl:        AdminSessionTTL,
	}, nil
}

func (m *AdminSessionManager) Issue(subject string) (string, *http.Cookie, error) {
	if subject == "" {
		return "", nil, fmt.Errorf("admin session subject is required")
	}
	now := m.now().UTC()
	claims := adminClaims{Subject: subject, IssuedAt: now.Unix(), ExpiresAt: now.Add(m.ttl).Unix()}
	token, err := m.sign(claims)
	if err != nil {
		return "", nil, err
	}
	return token, &http.Cookie{
		Name:     AdminSessionCookieName,
		Value:    token,
		Path:     "/admin",
		Expires:  now.Add(m.ttl),
		MaxAge:   int(m.ttl.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}, nil
}

func (m *AdminSessionManager) Verify(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", ErrInvalidSession
	}
	signed := parts[0] + "." + parts[1]
	expected := signBytes([]byte(signed), m.signingKey)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", ErrInvalidSession
	}
	if hmac.Equal(got, expected) == false {
		return "", ErrInvalidSession
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrInvalidSession
	}
	var claims adminClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", ErrInvalidSession
	}
	if claims.Subject == "" || m.now().UTC().Unix() >= claims.ExpiresAt {
		return "", ErrInvalidSession
	}
	return claims.Subject, nil
}

func (m *AdminSessionManager) VerifyCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie(AdminSessionCookieName)
	if err != nil {
		return "", ErrInvalidSession
	}
	return m.Verify(cookie.Value)
}

func (m *AdminSessionManager) sign(claims adminClaims) (string, error) {
	headerRaw, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payloadRaw, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal admin JWT claims: %w", err)
	}
	header := base64.RawURLEncoding.EncodeToString(headerRaw)
	payload := base64.RawURLEncoding.EncodeToString(payloadRaw)
	signed := header + "." + payload
	signature := base64.RawURLEncoding.EncodeToString(signBytes([]byte(signed), m.signingKey))
	return signed + "." + signature, nil
}

func signBytes(data, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func deriveAdminSigningKey(masterKey []byte) ([]byte, error) {
	reader := hkdf.New(sha256.New, masterKey, nil, []byte("sigilbridge admin jwt hs256 v1"))
	out := make([]byte, 32)
	if _, err := io.ReadFull(reader, out); err != nil {
		return nil, fmt.Errorf("derive admin JWT signing key: %w", err)
	}
	return out, nil
}
