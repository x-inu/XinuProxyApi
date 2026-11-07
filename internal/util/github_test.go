package util

import (
	"testing"

	"github.com/x-inu/XinuGate/v7/internal/githubauth"
)

func TestResolveGitHubToken(t *testing.T) {
	t.Cleanup(func() { githubauth.SetToken("") })
	githubauth.SetToken("")

	tests := []struct {
		name          string
		githubToken   string
		lowerToken    string
		gitStoreToken string
		gitStoreURL   string
		want          string
	}{
		{
			name:          "GITHUB_TOKEN is used",
			githubToken:   " primary-token ",
			lowerToken:    "lower-token",
			gitStoreToken: "gitstore-token",
			gitStoreURL:   "https://github.com/example/repo.git",
			want:          "primary-token",
		},
		{
			name:          "lowercase token is ignored",
			lowerToken:    " lower-token ",
			gitStoreToken: "gitstore-token",
			gitStoreURL:   "https://github.com/example/repo.git",
		},
		{
			name:          "Git store token is ignored for GitHub",
			gitStoreToken: " gitstore-token ",
			gitStoreURL:   "HTTPS://GITHUB.COM/example/repo.git",
		},
		{
			name: "no token configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", tt.githubToken)
			t.Setenv("github_token", tt.lowerToken)
			t.Setenv("GITSTORE_GIT_TOKEN", tt.gitStoreToken)
			t.Setenv("GITSTORE_GIT_URL", tt.gitStoreURL)
			githubauth.SetToken("")

			if got := ResolveGitHubToken(); got != tt.want {
				t.Fatalf("ResolveGitHubToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
