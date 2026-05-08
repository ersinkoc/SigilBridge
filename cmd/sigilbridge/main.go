package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sigilbridge/sigilbridge/cmd/sigilbridge/commands"
	"github.com/sigilbridge/sigilbridge/internal/auth"
	"github.com/sigilbridge/sigilbridge/internal/oauth"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: sigilbridge <version|init|serve|keys|oauth|pricing|backup|restore|maintenance>")
	}
	switch args[0] {
	case "version":
		fmt.Println(commands.Version(commands.VersionInfo{Version: version, Commit: commit, BuildDate: date}))
	case "init":
		dir := valueFlag(args[1:], "--dir", "-d")
		result, err := commands.InitConfig(dir, hasFlag(args[1:], "--force"))
		if err != nil {
			return err
		}
		fmt.Printf("created %s\ncreated %s\ncreated %s\ncreated %s\nadmin token: %s\n", result.ConfigPath, result.PoolsPath, result.OAuthProvidersPath, result.AdminTokensPath, result.AdminToken)
	case "keys":
		if len(args) >= 2 && args[1] == "create" {
			opts := parseKeysCreateArgs(args[2:])
			if opts.prefix == "" {
				opts.prefix = auth.PrefixTest
			}
			if opts.configPath != "" {
				plain, id, hash, err := commands.KeysCreateStored(context.Background(), opts.configPath, opts.prefix, opts.name)
				if err != nil {
					return err
				}
				fmt.Printf("%s\n%s\n%s\n", plain, id, hash)
				return nil
			}
			plain, hash, err := commands.KeysCreate(opts.prefix)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n%s\n", plain, hash)
			return nil
		}
		if len(args) >= 2 && args[1] == "list" {
			configPath := configFlag(args[2:])
			if configPath == "" {
				return fmt.Errorf("usage: sigilbridge keys list --config path")
			}
			keys, err := commands.KeysListStored(context.Background(), configPath)
			if err != nil {
				return err
			}
			for _, key := range keys {
				status := "active"
				if !key.RevokedAt.IsZero() {
					status = "revoked"
				}
				fmt.Printf("%s\t%s\t%s\t%s\n", key.ID, key.Name, status, key.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
			}
			return nil
		}
		if len(args) >= 3 && args[1] == "revoke" {
			configPath := configFlag(args[3:])
			if configPath == "" {
				return fmt.Errorf("usage: sigilbridge keys revoke <id> --config path")
			}
			return commands.KeysRevokeStored(context.Background(), configPath, args[2])
		}
		return fmt.Errorf("usage: sigilbridge keys create|list|revoke")
	case "oauth":
		return runOAuth(context.Background(), args[1:])
	case "pricing":
		if len(args) >= 2 && args[1] == "show" {
			raw, err := commands.PricingShow()
			if err != nil {
				return err
			}
			fmt.Println(string(raw))
			return nil
		}
		if len(args) >= 2 && args[1] == "update" {
			source := valueFlag(args[2:], "--source", "-s")
			output := valueFlag(args[2:], "--output", "-o")
			if source == "" && len(args) >= 3 && args[2] != "--output" && args[2] != "-o" {
				source = args[2]
			}
			return commands.PricingUpdate(source, output)
		}
		return fmt.Errorf("usage: sigilbridge pricing show|update")
	case "backup":
		configPath := configFlag(args[1:])
		output := valueFlag(args[1:], "--output", "-o")
		if configPath == "" || output == "" {
			return fmt.Errorf("usage: sigilbridge backup --config path --output path.db")
		}
		return commands.BackupConfig(context.Background(), configPath, output)
	case "restore":
		configPath := configFlag(args[1:])
		from := valueFlag(args[1:], "--from", "-f")
		if configPath == "" || from == "" {
			return fmt.Errorf("usage: sigilbridge restore --config path --from path.db")
		}
		return commands.RestoreConfig(context.Background(), configPath, from)
	case "maintenance":
		if len(args) >= 2 && args[1] == "vacuum" {
			configPath := configFlag(args[2:])
			if configPath == "" {
				return fmt.Errorf("usage: sigilbridge maintenance vacuum --config path")
			}
			return commands.MaintenanceVacuumConfig(context.Background(), configPath)
		}
		if len(args) >= 2 && args[1] == "prune-audit" {
			configPath := configFlag(args[2:])
			if configPath == "" {
				return fmt.Errorf("usage: sigilbridge maintenance prune-audit --config path")
			}
			return commands.MaintenancePruneAuditConfig(context.Background(), configPath, time.Now().UTC())
		}
		return fmt.Errorf("usage: sigilbridge maintenance vacuum|prune-audit --config path")
	case "serve":
		ctx, stop, reload := serveSignals(context.Background())
		defer stop()
		return commands.ServeConfig(ctx, configArg(args[1:]), commands.WithReloadTrigger(reload))
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

func runOAuth(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: sigilbridge oauth add|list|show|refresh|revoke --config path")
	}
	configPath := configFlag(args)
	if configPath == "" {
		return fmt.Errorf("usage: sigilbridge oauth %s --config path", args[0])
	}
	manager, cleanup, err := commands.OpenOAuthManagerConfig(configPath)
	if err != nil {
		return err
	}
	defer cleanup()
	switch args[0] {
	case "add":
		providerID := firstPositional(args[1:])
		if providerID == "" {
			return fmt.Errorf("usage: sigilbridge oauth add <provider-id> --config path [--name name] [--device]")
		}
		name := valueFlag(args[1:], "--name", "-n")
		var result oauth.BootstrapResult
		if hasFlag(args[1:], "--device") {
			result, err = manager.BootstrapDevice(ctx, providerID, name, func(result oauth.BootstrapResult) {
				fmt.Println(commands.FormatOAuthBootstrap(result))
			})
		} else {
			result, err = manager.BootstrapBrowser(ctx, providerID, name, true)
		}
		if err != nil {
			return err
		}
		fmt.Println(commands.FormatOAuthBootstrap(result))
		return nil
	case "list":
		ids, err := commands.OAuthList(ctx, manager)
		if err != nil {
			return err
		}
		for _, id := range ids {
			fmt.Println(id)
		}
		return nil
	case "show":
		id := firstPositional(args[1:])
		if id == "" {
			return fmt.Errorf("usage: sigilbridge oauth show <vault-id> --config path")
		}
		token, err := commands.OAuthShow(ctx, manager, id)
		if err != nil {
			return err
		}
		printOAuthToken(token)
		return nil
	case "refresh":
		id := firstPositional(args[1:])
		if id == "" {
			return fmt.Errorf("usage: sigilbridge oauth refresh <vault-id> --config path")
		}
		token, err := commands.OAuthRefresh(ctx, manager, id)
		if err != nil {
			return err
		}
		printOAuthToken(token)
		return nil
	case "revoke":
		id := firstPositional(args[1:])
		if id == "" {
			return fmt.Errorf("usage: sigilbridge oauth revoke <vault-id> --config path")
		}
		return commands.OAuthRevoke(ctx, manager, id)
	default:
		return fmt.Errorf("usage: sigilbridge oauth add|list|show|refresh|revoke --config path")
	}
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func firstPositional(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--config", "-c", "--name", "-n", "--source", "-s", "--output", "-o", "--from", "-f":
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

func printOAuthToken(token oauth.Token) {
	if !token.ExpiresAt.IsZero() {
		fmt.Printf("expires_at\t%s\n", token.ExpiresAt.Format(time.RFC3339))
	}
	if token.Scope != "" {
		fmt.Printf("scope\t%s\n", token.Scope)
	}
	if token.TokenType != "" {
		fmt.Printf("token_type\t%s\n", token.TokenType)
	}
}

func configFlag(args []string) string {
	return valueFlag(args, "--config", "-c")
}

func valueFlag(args []string, long, short string) string {
	for i := 0; i < len(args); i++ {
		if (args[i] == long || args[i] == short) && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

type keysCreateArgs struct {
	prefix     string
	configPath string
	name       string
}

func parseKeysCreateArgs(args []string) keysCreateArgs {
	var opts keysCreateArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config", "-c":
			if i+1 < len(args) {
				opts.configPath = args[i+1]
				i++
			}
		case "--name":
			if i+1 < len(args) {
				opts.name = args[i+1]
				i++
			}
		case auth.PrefixLive, auth.PrefixTest:
			opts.prefix = args[i]
		}
	}
	return opts
}

func configArg(args []string) string {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config", "-c":
			if i+1 < len(args) {
				return args[i+1]
			}
		default:
			if i == 0 {
				return args[i]
			}
		}
	}
	return ""
}
