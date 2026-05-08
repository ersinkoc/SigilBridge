package apikey

import (
	"net/url"
	"strings"
)

func openAIChatCompletionsURL(base string) string {
	return providerEndpoint(base, "/v1/chat/completions", "/chat/completions")
}

func anthropicMessagesURL(base string) string {
	return providerEndpoint(base, "/v1/messages", "/messages")
}

func anthropicCountTokensURL(base string) string {
	return providerEndpoint(base, "/v1/messages/count_tokens", "/messages/count_tokens")
}

func providerEndpoint(base, defaultPath, versionedPath string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return defaultPath
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if hasVersionSuffix(base) {
			return base + versionedPath
		}
		return base + defaultPath
	}
	path := strings.TrimRight(u.Path, "/")
	if hasVersionSuffix(path) {
		u.Path = path + versionedPath
	} else {
		u.Path = path + defaultPath
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func hasVersionSuffix(path string) bool {
	path = strings.TrimRight(path, "/")
	if path == "" {
		return false
	}
	lastSlash := strings.LastIndex(path, "/")
	last := path
	if lastSlash >= 0 {
		last = path[lastSlash+1:]
	}
	if len(last) < 2 || last[0] != 'v' {
		return false
	}
	for _, r := range last[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
