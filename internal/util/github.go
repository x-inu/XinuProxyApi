package util

import "github.com/x-inu/XinuProxyApi/v7/internal/githubauth"

// ResolveGitHubToken returns the GitHub API token in priority order:
// 1. server.github-token (configured)
// 2. GITHUB_TOKEN
func ResolveGitHubToken() string {
	return githubauth.ResolveToken()
}
