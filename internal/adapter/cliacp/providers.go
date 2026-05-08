package cliacp

func NewClaudeCode(command string, args ...string) Adapter {
	if command == "" {
		command = "claude"
	}
	return New("claude_code_cli", command, args...)
}

func NewCodex(command string, args ...string) Adapter {
	if command == "" {
		command = "codex"
	}
	return New("codex_cli", command, args...)
}

func NewGemini(command string, args ...string) Adapter {
	if command == "" {
		command = "gemini"
	}
	return New("gemini_cli", command, args...)
}

func NewAider(command string, args ...string) Adapter {
	if command == "" {
		command = "aider"
	}
	return New("aider_cli", command, args...)
}
