package githubauth

import "testing"

func TestGlobalToken(t *testing.T) {
	t.Cleanup(func() { SetToken("") })
	t.Setenv("GITHUB_TOKEN", "env-token")
	t.Setenv("github_token", "legacy-lowercase-token")
	t.Setenv("GITSTORE_GIT_URL", "https://github.com/example/repo.git")
	t.Setenv("GITSTORE_GIT_TOKEN", "gitstore-token")
	SetToken(" config-token ")
	if ResolveToken() != "config-token" {
		t.Fatal("configuration must take precedence")
	}
	for _, raw := range []string{"https://api.github.com/repos/a/b", "https://API.GITHUB.COM:443/repos/a/b"} {
		if TokenForURL(raw) != "config-token" {
			t.Fatalf("missing token for %s", raw)
		}
	}
	for _, raw := range []string{"http://api.github.com/", "https://api.github.com:444/", "https://api.github.com.evil.example/", "https://github.com/", "https://raw.githubusercontent.com/", "https://user@api.github.com/", ":invalid"} {
		if TokenForURL(raw) != "" {
			t.Fatalf("token leaked to %s", raw)
		}
	}
	SetToken("replacement")
	if ResolveToken() != "replacement" {
		t.Fatal("reload did not replace token")
	}
	SetToken("")
	if ResolveToken() != "env-token" {
		t.Fatal("clearing configuration must restore GITHUB_TOKEN fallback")
	}

	// Legacy fallbacks are intentionally unsupported: only config -> GITHUB_TOKEN.
	t.Setenv("GITHUB_TOKEN", "")
	SetToken("")
	if got := ResolveToken(); got != "" {
		t.Fatalf("legacy github_token/GITSTORE fallback must be ignored, got %q", got)
	}
}
