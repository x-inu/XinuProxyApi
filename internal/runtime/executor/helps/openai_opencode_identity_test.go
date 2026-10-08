package helps

import (
	"net/http"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func opencodeTestCompat(enabled bool) *config.OpenAICompatibility {
	return &config.OpenAICompatibility{
		Name:                       "opencodeFree",
		BaseURL:                    "https://opencode.ai/zen/v1",
		SynthesizeOpencodeIdentity: &enabled,
	}
}

func TestOpencodeIdentityEnabled(t *testing.T) {
	if OpencodeIdentityEnabled(nil) {
		t.Fatal("nil compat must be disabled")
	}
	if OpencodeIdentityEnabled(&config.OpenAICompatibility{}) {
		t.Fatal("omitted flag must be disabled")
	}
	if OpencodeIdentityEnabled(opencodeTestCompat(false)) {
		t.Fatal("explicit false must be disabled")
	}
	if !OpencodeIdentityEnabled(opencodeTestCompat(true)) {
		t.Fatal("explicit true must be enabled")
	}
}

func TestIsOpencodeFreeModel(t *testing.T) {
	for _, model := range []string{"space-bunny-free", "mimo-v2.6-flash-free", "x-free"} {
		if !IsOpencodeFreeModel(model) {
			t.Fatalf("model %q must be free", model)
		}
	}
	for _, model := range []string{"gpt-5.4", "big-pickle", "", "free"} {
		if IsOpencodeFreeModel(model) {
			t.Fatalf("model %q must be premium", model)
		}
	}
}

func TestOpencodeFreeTierRejection(t *testing.T) {
	enabled := opencodeTestCompat(true)
	if _, rejected := OpencodeFreeTierRejection(nil, "gpt-5.4", ""); rejected {
		t.Fatal("nil compat must not reject")
	}
	if _, rejected := OpencodeFreeTierRejection(enabled, "gpt-5.4", "sk-zen"); rejected {
		t.Fatal("keyed credential must not reject")
	}
	if _, rejected := OpencodeFreeTierRejection(enabled, "space-bunny-free", ""); rejected {
		t.Fatal("free model must not reject")
	}
	msg, rejected := OpencodeFreeTierRejection(enabled, "gpt-5.4", "")
	if !rejected || !strings.Contains(msg, "gpt-5.4") {
		t.Fatalf("premium keyless model must reject with model name, got %q", msg)
	}
	disabled := opencodeTestCompat(false)
	if _, rejected := OpencodeFreeTierRejection(disabled, "gpt-5.4", ""); rejected {
		t.Fatal("disabled flag must not reject")
	}
}

func TestApplyOpencodeIdentity(t *testing.T) {
	compat := opencodeTestCompat(true)
	payload := []byte(`{"model":"space-bunny-free","messages":[{"role":"user","content":"hi"}]}`)

	first, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyOpencodeIdentity(first, compat, payload)
	if got := first.Header.Get("User-Agent"); got != "opencode" {
		t.Fatalf("User-Agent = %q, want opencode", got)
	}
	if got := first.Header.Get("x-opencode-client"); got != "desktop" {
		t.Fatalf("x-opencode-client = %q, want desktop", got)
	}
	if got := first.Header.Get("x-opencode-project"); got != "global" {
		t.Fatalf("x-opencode-project = %q, want global", got)
	}
	requestID := first.Header.Get("x-opencode-request")
	if requestID == "" {
		t.Fatal("x-opencode-request must be set")
	}
	sessionID := first.Header.Get("x-opencode-session")
	if sessionID == "" {
		t.Fatal("x-opencode-session must be derived from payload")
	}

	second, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyOpencodeIdentity(second, compat, payload)
	if second.Header.Get("x-opencode-request") == requestID {
		t.Fatal("x-opencode-request must be unique per request")
	}
	if second.Header.Get("x-opencode-session") != sessionID {
		t.Fatal("x-opencode-session must be stable for the same body")
	}

	// Existing values win, except the per-request id.
	third, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	third.Header.Set("x-opencode-client", "custom")
	third.Header.Set("x-opencode-session", "pinned")
	ApplyOpencodeIdentity(third, compat, payload)
	if got := third.Header.Get("x-opencode-client"); got != "custom" {
		t.Fatalf("existing client header must win, got %q", got)
	}
	if got := third.Header.Get("x-opencode-session"); got != "pinned" {
		t.Fatalf("existing session header must win, got %q", got)
	}

	// Disabled compat leaves the request untouched.
	plain, err := http.NewRequest(http.MethodPost, "https://example.com/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyOpencodeIdentity(plain, opencodeTestCompat(false), payload)
	if got := plain.Header.Get("x-opencode-request"); got != "" {
		t.Fatalf("disabled compat must not add headers, got %q", got)
	}
}

func TestOpencodeSessionID(t *testing.T) {
	if got := OpencodeSessionID(nil); got != "" {
		t.Fatalf("nil payload must yield empty session, got %q", got)
	}
	if got := OpencodeSessionID([]byte(`{}`)); got != "" {
		t.Fatalf("empty object must yield empty session, got %q", got)
	}
	a := OpencodeSessionID([]byte(`{"model":"m","messages":[{"role":"user","content":"hello"}]}`))
	b := OpencodeSessionID([]byte(`{"model":"m","messages":[{"role":"user","content":"hello"}]}`))
	if a == "" || a != b {
		t.Fatalf("same body must yield same non-empty session, got %q vs %q", a, b)
	}
	c := OpencodeSessionID([]byte(`{"model":"m","messages":[{"role":"user","content":"different"}]}`))
	if c == a {
		t.Fatal("different bodies must yield different sessions")
	}
}
