package config

import "testing"

func TestValidatePoolsAllowsAPIKeyWithoutMasterKey(t *testing.T) {
	t.Setenv("SIGILBRIDGE_MASTER_KEY", "")
	err := ValidatePools([]Pool{{
		Name: "sonnet",
		Upstreams: []Upstream{{
			ID:       "anth-a",
			Provider: "anthropic_api",
			Config:   map[string]any{"api_key": "test"},
		}},
	}}, "SIGILBRIDGE_MASTER_KEY")
	if err != nil {
		t.Fatalf("ValidatePools() error = %v", err)
	}
}

func TestValidatePoolsRejectsOAuthWithoutMasterKey(t *testing.T) {
	t.Setenv("SIGILBRIDGE_MASTER_KEY", "")
	err := ValidatePools([]Pool{{
		Name: "sonnet",
		Upstreams: []Upstream{{
			ID:       "claude-max",
			Provider: "claude_oauth",
			Config:   map[string]any{"token_ref": "vault://claude/max"},
		}},
	}}, "SIGILBRIDGE_MASTER_KEY")
	if err == nil {
		t.Fatal("ValidatePools() error = nil, want missing master key error")
	}
}

func TestPoolsDecodeMappingForm(t *testing.T) {
	t.Setenv("SIGILBRIDGE_MASTER_KEY", "present")
	pools, err := LoadPoolsFromBytes([]byte(`
pools:
  sonnet:
    strategy: priority_first
    upstreams:
      - id: claude-max
        provider: claude_oauth
        config:
          token_ref: vault://claude/max
`), "SIGILBRIDGE_MASTER_KEY")
	if err != nil {
		t.Fatalf("LoadPoolsFromBytes() error = %v", err)
	}
	if len(pools.Pools) != 1 || pools.Pools[0].Name != "sonnet" {
		t.Fatalf("decoded pools = %#v", pools.Pools)
	}
}
