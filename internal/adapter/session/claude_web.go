package session

func NewClaudeWeb(baseURL string, vault Vault) *WebAdapter {
	if baseURL == "" {
		baseURL = "https://claude.ai"
	}
	return NewWebAdapter("claude_web", baseURL, "/api/organizations/{organization_id}/chat_conversations/completion", "anthropic", vault)
}
