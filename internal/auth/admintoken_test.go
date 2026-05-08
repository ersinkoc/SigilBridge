package auth

import "testing"

func TestAdminTokensVerify(t *testing.T) {
	store, err := ParseAdminTokens([]byte(`
tokens:
  - name: active
    token: alpha
  - name: revoked
    token: beta
    revoked: true
`))
	if err != nil {
		t.Fatalf("ParseAdminTokens() error = %v", err)
	}
	if got, ok := store.VerifyHeader("Bearer alpha"); !ok || got.Name != "active" {
		t.Fatalf("VerifyHeader(active) = %#v, %v", got, ok)
	}
	if _, ok := store.Verify("beta"); ok {
		t.Fatal("Verify(revoked) succeeded")
	}
	if _, ok := store.Verify("bad"); ok {
		t.Fatal("Verify(bad) succeeded")
	}
}
