package commands

import (
	"context"
	"fmt"

	"github.com/sigilbridge/sigilbridge/internal/config"
	"github.com/sigilbridge/sigilbridge/internal/oauth"
	"github.com/sigilbridge/sigilbridge/internal/vault"
)

type OAuthManager interface {
	Bootstrap(ctx context.Context, providerID, name, mode string) (oauth.BootstrapResult, error)
	Refresh(ctx context.Context, id string) (oauth.Token, error)
	Revoke(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (oauth.Token, error)
	List(ctx context.Context) ([]string, error)
}

func OAuthAdd(ctx context.Context, manager OAuthManager, providerID, name string, device bool) (oauth.BootstrapResult, error) {
	mode := "browser"
	if device {
		mode = "device"
	}
	return manager.Bootstrap(ctx, providerID, name, mode)
}

func OAuthRefresh(ctx context.Context, manager OAuthManager, vaultID string) (oauth.Token, error) {
	return manager.Refresh(ctx, vaultID)
}

func OAuthRevoke(ctx context.Context, manager OAuthManager, vaultID string) error {
	return manager.Revoke(ctx, vaultID)
}

func OAuthShow(ctx context.Context, manager OAuthManager, vaultID string) (oauth.Token, error) {
	return manager.Get(ctx, vaultID)
}

func OAuthList(ctx context.Context, manager OAuthManager) ([]string, error) {
	return manager.List(ctx)
}

func FormatOAuthBootstrap(result oauth.BootstrapResult) string {
	if result.Token.AccessToken != "" {
		return fmt.Sprintf("Stored OAuth credential %s", result.VaultID)
	}
	if result.Mode == "device" {
		verificationURI := result.VerificationURI
		if result.VerificationURIComplete != "" {
			verificationURI = result.VerificationURIComplete
		}
		return fmt.Sprintf("Open %s and enter code %s", verificationURI, result.UserCode)
	}
	return result.AuthURL
}

func OpenOAuthManagerConfig(configPath string) (*oauth.Manager, func(), error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, err
	}
	db, err := openConfiguredDB(configPath)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = db.Close()
	}
	masterKey, err := vault.LoadMasterKeyFromEnv(cfg.Vault.MasterKeyEnv)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	key := masterKey.Bytes()
	tokenVault, err := vault.New(db, key)
	for i := range key {
		key[i] = 0
	}
	masterKey.Wipe()
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	cleanup = func() {
		tokenVault.Close()
		_ = db.Close()
	}
	providersPath := config.ResolveRelative(configPath, cfg.OAuth.ProvidersFile)
	registry, err := oauth.LoadRegistry(providersPath)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return oauth.NewManager(registry, tokenVault, nil), cleanup, nil
}
