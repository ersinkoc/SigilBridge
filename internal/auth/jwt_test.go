package auth

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdminSessionIssueVerifyCookie(t *testing.T) {
	manager, err := NewAdminSessionManager(make([]byte, 32))
	if err != nil {
		t.Fatalf("NewAdminSessionManager() error = %v", err)
	}
	now := testTime()
	manager.now = func() time.Time { return now }

	token, cookie, err := manager.Issue("admin")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if cookie == nil || !cookie.HttpOnly || !cookie.Secure || cookie.MaxAge != int(AdminSessionTTL.Seconds()) {
		t.Fatalf("bad cookie: %#v", cookie)
	}
	subject, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if subject != "admin" {
		t.Fatalf("subject = %q", subject)
	}
	req := httptest.NewRequest("GET", "/admin/v1/keys", nil)
	req.AddCookie(cookie)
	if subject, err := manager.VerifyCookie(req); err != nil || subject != "admin" {
		t.Fatalf("VerifyCookie() = %q, %v", subject, err)
	}
}

func TestAdminSessionRejectsExpiredAndTampered(t *testing.T) {
	manager, err := NewAdminSessionManager(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	now := testTime()
	manager.now = func() time.Time { return now }
	token, _, err := manager.Issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return now.Add(16 * time.Minute) }
	if _, err := manager.Verify(token); err == nil {
		t.Fatal("Verify(expired) error = nil")
	}
	parts := strings.Split(token, ".")
	parts[1] = parts[1] + "x"
	manager.now = func() time.Time { return now }
	if _, err := manager.Verify(strings.Join(parts, ".")); err == nil {
		t.Fatal("Verify(tampered) error = nil")
	}
}
