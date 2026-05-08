package session

func NewChatGPTWeb(baseURL string, vault Vault) *WebAdapter {
	if baseURL == "" {
		baseURL = "https://chatgpt.com"
	}
	return NewWebAdapter("chatgpt_web", baseURL, "/backend-api/conversation", "openai", vault)
}
