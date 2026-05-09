package cliacp

type DefaultAgent struct {
	ID         string
	Name       string
	Command    string
	Protocol   string
	Framing    string
	Args       []string
	AuthStatus string
	Source     string
	Version    string
}

func Defaults() []DefaultAgent {
	return []DefaultAgent{
		{ID: "claude_code_cli", Name: "Claude Code", Command: "claude", Protocol: "headless", AuthStatus: "Uses Claude Code's local login or configured API key helper", Source: "built-in"},
		{ID: "codex_cli", Name: "Codex CLI", Command: "codex", Protocol: "headless", AuthStatus: "Uses Codex CLI's local login", Source: "built-in"},
		{ID: "gemini_cli", Name: "Gemini CLI", Command: "gemini", Protocol: "acp", Framing: "ndjson", Args: []string{"--acp", "--skip-trust"}, AuthStatus: "Uses Gemini CLI's local login", Source: "built-in"},
		{ID: "aider_cli", Name: "Aider", Command: "aider", Protocol: "headless", AuthStatus: "Uses Aider's local environment and provider credentials", Source: "built-in"},
		{ID: "agoragentic-acp", Name: "Agoragentic", Command: "npx", Protocol: "acp", Args: []string{"-y", "agoragentic-mcp@1.3.0", "--acp"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "1.3.0"},
		{ID: "amp-acp", Name: "Amp", Command: "amp-acp", Protocol: "acp", AuthStatus: "Requires local Amp ACP binary from the ACP registry", Source: "ACP registry", Version: "0.7.0"},
		{ID: "auggie", Name: "Auggie CLI", Command: "npx", Protocol: "acp", Args: []string{"-y", "@augmentcode/auggie@0.26.0", "--acp"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "0.26.0"},
		{ID: "autohand", Name: "Autohand Code", Command: "npx", Protocol: "acp", Args: []string{"-y", "@autohandai/autohand-acp@0.2.1"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "0.2.1"},
		{ID: "claude-acp", Name: "Claude Agent", Command: "npx", Protocol: "acp", Args: []string{"-y", "@agentclientprotocol/claude-agent-acp@0.33.1"}, AuthStatus: "Runs Claude Agent SDK through the ACP registry adapter", Source: "ACP registry", Version: "0.33.1"},
		{ID: "cline", Name: "Cline", Command: "npx", Protocol: "acp", Args: []string{"-y", "cline@2.18.0", "--acp"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "2.18.0"},
		{ID: "codebuddy-code", Name: "Codebuddy Code", Command: "npx", Protocol: "acp", Args: []string{"-y", "@tencent-ai/codebuddy-code@2.95.1", "--acp"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "2.95.1"},
		{ID: "codex-acp", Name: "Codex CLI", Command: "npx", Protocol: "acp", Args: []string{"-y", "@zed-industries/codex-acp@0.14.0"}, AuthStatus: "Runs Codex CLI through the ACP registry adapter", Source: "ACP registry", Version: "0.14.0"},
		{ID: "cortex-code", Name: "Cortex Code", Command: "cortex", Protocol: "acp", AuthStatus: "Requires local Cortex Code ACP binary", Source: "ACP registry", Version: "1.0.73"},
		{ID: "corust-agent", Name: "Corust Agent", Command: "corust-agent-acp", Protocol: "acp", AuthStatus: "Requires local Corust ACP binary", Source: "ACP registry", Version: "0.5.1"},
		{ID: "crow-cli", Name: "crow-cli", Command: "crow-cli", Protocol: "acp", AuthStatus: "Requires local crow-cli ACP binary", Source: "ACP registry", Version: "0.1.23"},
		{ID: "cursor", Name: "Cursor", Command: "cursor-agent", Protocol: "acp", AuthStatus: "Requires local Cursor agent binary", Source: "ACP registry", Version: "2026.03.30"},
		{ID: "deepagents", Name: "DeepAgents", Command: "npx", Protocol: "acp", Args: []string{"-y", "deepagents-acp@0.1.7"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "0.1.7"},
		{ID: "dimcode", Name: "DimCode", Command: "npx", Protocol: "acp", Args: []string{"-y", "dimcode@0.0.66", "acp"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "0.0.66"},
		{ID: "dirac", Name: "Dirac", Command: "npx", Protocol: "acp", Args: []string{"-y", "dirac-cli@0.3.41", "--acp"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "0.3.41"},
		{ID: "factory-droid", Name: "Factory Droid", Command: "npx", Protocol: "acp", Args: []string{"-y", "droid@0.122.0", "exec", "--output-format", "acp-daemon"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "0.122.0"},
		{ID: "gemini", Name: "Gemini CLI", Command: "npx", Protocol: "acp", Args: []string{"-y", "@google/gemini-cli@0.41.2", "--acp"}, AuthStatus: "Google Gemini CLI through the ACP registry package", Source: "ACP registry", Version: "0.41.2"},
		{ID: "github-copilot-cli", Name: "GitHub Copilot", Command: "npx", Protocol: "acp", Args: []string{"-y", "@github/copilot@1.0.44", "--acp"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "1.0.44"},
		{ID: "glm-acp-agent", Name: "GLM Agent", Command: "npx", Protocol: "acp", Args: []string{"-y", "glm-acp-agent@1.1.2"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "1.1.2"},
		{ID: "goose", Name: "Goose", Command: "goose", Protocol: "acp", AuthStatus: "Requires local Goose ACP binary", Source: "ACP registry", Version: "1.33.1"},
		{ID: "junie", Name: "Junie", Command: "junie", Protocol: "acp", AuthStatus: "Requires local Junie binary", Source: "ACP registry", Version: "1543.24.0"},
		{ID: "kilo", Name: "Kilo", Command: "npx", Protocol: "acp", Args: []string{"-y", "@kilocode/cli@7.2.40", "acp"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "7.2.40"},
		{ID: "kimi", Name: "Kimi CLI", Command: "kimi", Protocol: "acp", AuthStatus: "Requires local Kimi CLI binary", Source: "ACP registry", Version: "1.41.0"},
		{ID: "mistral-vibe", Name: "Mistral Vibe", Command: "vibe-acp", Protocol: "acp", AuthStatus: "Requires local Mistral Vibe ACP binary", Source: "ACP registry", Version: "2.9.3"},
		{ID: "nova", Name: "Nova", Command: "npx", Protocol: "acp", Args: []string{"-y", "@compass-ai/nova@1.1.7", "acp"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "1.1.7"},
		{ID: "opencode", Name: "OpenCode", Command: "opencode", Protocol: "acp", AuthStatus: "Requires local OpenCode binary", Source: "ACP registry", Version: "1.14.41"},
		{ID: "pi-acp", Name: "pi ACP", Command: "npx", Protocol: "acp", Args: []string{"-y", "pi-acp@0.0.26"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "0.0.26"},
		{ID: "poolside", Name: "Poolside", Command: "pool", Protocol: "acp", AuthStatus: "Requires local Poolside ACP binary", Source: "ACP registry", Version: "1.0.0"},
		{ID: "qoder", Name: "Qoder CLI", Command: "npx", Protocol: "acp", Args: []string{"-y", "@qoder-ai/qodercli@0.2.11", "--acp"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "0.2.11"},
		{ID: "qwen-code", Name: "Qwen Code", Command: "npx", Protocol: "acp", Args: []string{"-y", "@qwen-code/qwen-code@0.15.9", "--acp", "--experimental-skills"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "0.15.9"},
		{ID: "sigit", Name: "siGit Code", Command: "npx", Protocol: "acp", Args: []string{"-y", "@smbcloud/sigit@1.0.3"}, AuthStatus: "Runnable through ACP registry package", Source: "ACP registry", Version: "1.0.3"},
		{ID: "stakpak", Name: "Stakpak", Command: "stakpak", Protocol: "acp", AuthStatus: "Requires local Stakpak binary", Source: "ACP registry", Version: "0.3.80"},
		{ID: "vtcode", Name: "VT Code", Command: "vtcode", Protocol: "acp", AuthStatus: "Requires local VT Code binary", Source: "ACP registry", Version: "0.96.14"},
	}
}
