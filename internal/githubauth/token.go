// Package githubauth provides shared GitHub API credentials.
package githubauth

import (
	"net/url"
	"os"
	"strings"
	"sync/atomic"
)

var configuredToken atomic.Pointer[string]

// SetToken replaces the global configured token, including clearing it on reload.
func SetToken(token string) {
	token = strings.TrimSpace(token)
	configuredToken.Store(&token)
}

// ResolveToken returns the configured token, falling back to GITHUB_TOKEN.
func ResolveToken() string {
	if token := configuredToken.Load(); token != nil && *token != "" {
		return *token
	}
	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
}

// TokenForURL never exposes the shared credential outside the HTTPS GitHub API.
func TokenForURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || !strings.EqualFold(u.Hostname(), "api.github.com") || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return ""
	}
	return ResolveToken()
}
