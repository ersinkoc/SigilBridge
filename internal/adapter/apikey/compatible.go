package apikey

func NewGroq(baseURL string) OpenAI {
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai"
	}
	return NewOpenAI("groq", baseURL)
}

func NewMistral(baseURL string) OpenAI {
	if baseURL == "" {
		baseURL = "https://api.mistral.ai"
	}
	return NewOpenAI("mistral_api", baseURL)
}

func NewDeepSeek(baseURL string) OpenAI {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	return NewOpenAI("deepseek_api", baseURL)
}

func NewGemini(baseURL string) OpenAI {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	return NewOpenAI("gemini_api", baseURL)
}
