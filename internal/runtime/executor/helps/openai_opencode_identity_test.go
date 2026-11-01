package helps

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/tidwall/gjson"
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

func TestIsOpencodeLegacyOpenModel(t *testing.T) {
	for _, model := range []string{"space-bunny-free", "oc/space-bunny-free"} {
		if !IsOpencodeLegacyOpenModel(model) {
			t.Fatalf("model %q must be legacy-open", model)
		}
	}
	for _, model := range []string{"mimo-v2.6-flash-free", "big-pickle", ""} {
		if IsOpencodeLegacyOpenModel(model) {
			t.Fatalf("model %q must not be legacy-open", model)
		}
	}
}

func TestNeedsOpencodeFullRecipe(t *testing.T) {
	enabled := opencodeTestCompat(true)
	if NeedsOpencodeFullRecipe(nil, "mimo-v2.6-flash-free", "") {
		t.Fatal("nil compat must not need the recipe")
	}
	if NeedsOpencodeFullRecipe(enabled, "mimo-v2.6-flash-free", "sk-zen") {
		t.Fatal("keyed credential must not need the recipe")
	}
	if NeedsOpencodeFullRecipe(enabled, "space-bunny-free", "") {
		t.Fatal("legacy-open model must not need the recipe")
	}
	if NeedsOpencodeFullRecipe(enabled, "oc/space-bunny-free", "") {
		t.Fatal("prefixed legacy-open model must not need the recipe")
	}
	if NeedsOpencodeFullRecipe(enabled, "big-pickle", "") {
		t.Fatal("premium model must not need the recipe")
	}
	if !NeedsOpencodeFullRecipe(enabled, "mimo-v2.6-flash-free", "") {
		t.Fatal("keyless locked free model must need the recipe")
	}
	if !NeedsOpencodeFullRecipe(enabled, "oc/longcat-2.5-preview-free", "") {
		t.Fatal("prefixed keyless locked free model must need the recipe")
	}
	if NeedsOpencodeFullRecipe(opencodeTestCompat(false), "mimo-v2.6-flash-free", "") {
		t.Fatal("disabled flag must not need the recipe")
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

var opencodeSessionPattern = regexp.MustCompile(`^ses_[0-9A-Za-z]{26}$`)

func TestMintOpencodeSessionID(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 50; i++ {
		id := MintOpencodeSessionID()
		if !opencodeSessionPattern.MatchString(id) {
			t.Fatalf("minted session %q must match ses_ + 26 alnum", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("minted session %q must be unique", id)
		}
		seen[id] = struct{}{}
		// The first 12 chars encode the mint timestamp window; a random id
		// outside the window is rejected upstream.
		stamp := decodeOpencodeSessionForTest(t, id)
		if delta := time.Since(stamp); delta < 0 || delta > time.Hour {
			t.Fatalf("minted session %q must embed the current time, got %v ago", id, delta)
		}
	}
}

// decodeOpencodeSessionForTest inverts the 48-bit complement encoding so the
// test can assert the embedded timestamp is fresh.
func decodeOpencodeSessionForTest(t *testing.T, id string) time.Time {
	t.Helper()
	var head uint64
	for _, c := range id[len("ses_") : len("ses_")+12] {
		head <<= 4
		switch {
		case c >= '0' && c <= '9':
			head |= uint64(c - '0')
		case c >= 'a' && c <= 'f':
			head |= uint64(c-'a') + 10
		case c >= 'A' && c <= 'F':
			head |= uint64(c-'A') + 10
		default:
			t.Fatalf("session head %q must be hex", id)
		}
	}
	n := (uint64(1)<<48 - 1 - head) >> 12
	now := uint64(time.Now().UnixMilli())
	window := uint64(1) << 36
	// Rebuild the wall clock from the 36-bit windowed value.
	candidate := now&^(window-1) | n
	for candidate > now+uint64(time.Hour.Milliseconds()) {
		candidate -= window
	}
	return time.UnixMilli(int64(candidate))
}

func TestApplyOpencodeIdentity(t *testing.T) {
	compat := opencodeTestCompat(true)
	payload := []byte(`{"model":"mimo-v2.6-flash-free","messages":[{"role":"user","content":"hi"}]}`)

	first, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyOpencodeIdentity(first, compat, payload)
	if got := first.Header.Get("User-Agent"); got != "opencode/latest/2.2.5/cli" {
		t.Fatalf("User-Agent = %q, want versioned CLI agent", got)
	}
	if got := first.Header.Get("x-opencode-client"); got != "cli" {
		t.Fatalf("x-opencode-client = %q, want cli", got)
	}
	if got := first.Header.Get("x-opencode-project"); got != "global" {
		t.Fatalf("x-opencode-project = %q, want global", got)
	}
	if requestID := first.Header.Get("x-opencode-request"); requestID == "" {
		t.Fatal("x-opencode-request must be set")
	}
	sessionID := first.Header.Get("x-opencode-session")
	if !opencodeSessionPattern.MatchString(sessionID) {
		t.Fatalf("x-opencode-session = %q, want minted ses_ id", sessionID)
	}

	second, err := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyOpencodeIdentity(second, compat, payload)
	if second.Header.Get("x-opencode-request") == first.Header.Get("x-opencode-request") {
		t.Fatal("x-opencode-request must be unique per request")
	}
	if second.Header.Get("x-opencode-session") == sessionID {
		t.Fatal("x-opencode-session must be minted fresh per request")
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

func opencodeToolNames(payload []byte) []string {
	var names []string
	for _, tool := range gjson.GetBytes(payload, "tools").Array() {
		name := tool.Get("function.name").String()
		if name == "" {
			name = tool.Get("name").String()
		}
		names = append(names, name)
	}
	return names
}

func TestEnsureOpencodeMarkerTools(t *testing.T) {
	bare := []byte(`{"model":"mimo-v2.6-flash-free","messages":[{"role":"user","content":"hi"}]}`)
	filled := EnsureOpencodeMarkerTools(bare)
	names := opencodeToolNames(filled)
	if len(names) != 2 || names[0] != "read" || names[1] != "shell" {
		t.Fatalf("bare payload must gain read+shell markers, got %v", names)
	}
	if gjson.GetBytes(filled, "model").String() != "mimo-v2.6-flash-free" {
		t.Fatal("marker injection must preserve the model")
	}

	again := EnsureOpencodeMarkerTools(filled)
	if len(opencodeToolNames(again)) != 2 {
		t.Fatal("marker injection must be idempotent")
	}

	partial := []byte(`{"model":"m","tools":[{"type":"function","function":{"name":"shell","description":"x","parameters":{"type":"object","properties":{}}}}]}`)
	names = opencodeToolNames(EnsureOpencodeMarkerTools(partial))
	if len(names) != 2 || names[0] != "shell" || names[1] != "read" {
		t.Fatalf("partial tools must gain only the missing marker, got %v", names)
	}

	weird := []byte(`{"model":"m","tools":"nope"}`)
	if out := EnsureOpencodeMarkerTools(weird); string(out) != string(weird) {
		t.Fatal("non-array tools must be left untouched")
	}
}

func TestAssembleOpencodeStreamCompletion(t *testing.T) {
	sse := "data: {\"id\":\"a\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hel\"}}]}\n" +
		"data: {\"id\":\"a\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n" +
		"data: {\"id\":\"a\",\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n" +
		"data: [DONE]\n"
	out := AssembleOpencodeStreamCompletion([]byte(sse), "mimo-v2.6-flash-free")
	if gjson.GetBytes(out, "object").String() != "chat.completion" {
		t.Fatalf("assembled body must be a completion, got %s", out)
	}
	if got := gjson.GetBytes(out, "choices.0.message.content").String(); got != "Hello" {
		t.Fatalf("assembled content = %q, want Hello", got)
	}
	if got := gjson.GetBytes(out, "choices.0.finish_reason").String(); got != "stop" {
		t.Fatalf("assembled finish_reason = %q, want stop", got)
	}
	if got := gjson.GetBytes(out, "usage.total_tokens").Int(); got != 5 {
		t.Fatalf("assembled usage must be preserved, got total_tokens=%d", got)
	}

	plain := []byte(`{"error":"nope"}`)
	if out := AssembleOpencodeStreamCompletion(plain, "m"); string(out) != string(plain) {
		t.Fatal("non-SSE bodies must be returned untouched")
	}
}

func TestAssembleOpencodeStreamCompletionToolCalls(t *testing.T) {
	sse := "data: {\"id\":\"a\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"\"}}]}}]}\n" +
		"data: {\"id\":\"a\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"city\\\":\\\"Jakarta\\\"}\"}}]}}]}\n" +
		"data: {\"id\":\"a\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n" +
		"data: [DONE]\n"
	out := AssembleOpencodeStreamCompletion([]byte(sse), "mimo-v2.6-flash-free")
	call := gjson.GetBytes(out, "choices.0.message.tool_calls.0")
	if call.Get("id").String() != "call_1" {
		t.Fatalf("assembled tool call id = %q, got %s", call.Get("id").String(), out)
	}
	if call.Get("function.name").String() != "get_weather" {
		t.Fatalf("assembled tool name = %q, got %s", call.Get("function.name").String(), out)
	}
	if call.Get("function.arguments").String() != `{"city":"Jakarta"}` {
		t.Fatalf("assembled tool arguments = %q, got %s", call.Get("function.arguments").String(), out)
	}
	if got := gjson.GetBytes(out, "choices.0.finish_reason").String(); got != "tool_calls" {
		t.Fatalf("assembled finish_reason = %q, want tool_calls", got)
	}
	if gjson.GetBytes(out, "choices.0.message.tool_calls").String() == "" {
		t.Fatalf("tool_calls must be present, got %s", out)
	}
}

func TestOpencodeUpstreamModel(t *testing.T) {
	if got := OpencodeUpstreamModel([]byte(`{"model":"  mimo-v2.6-flash-free "}`)); got != "mimo-v2.6-flash-free" {
		t.Fatalf("upstream model = %q", got)
	}
	if got := OpencodeUpstreamModel([]byte(`{}`)); got != "" {
		t.Fatalf("missing model must yield empty string, got %q", got)
	}
}

func TestEnsureOpencodeSystemPrompt(t *testing.T) {
	// Case 1: no system message → prepend one with <env>
	bare := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)
	out := EnsureOpencodeSystemPrompt(bare)
	msgs := gjson.GetBytes(out, "messages").Array()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Get("role").String() != "system" {
		t.Fatal("first message must be system")
	}
	sys := msgs[0].Get("content").String()
	if !strings.Contains(sys, "<env>") {
		t.Fatal("system prompt must contain <env> block")
	}
	if !strings.Contains(sys, "OpenCode") {
		t.Fatal("system prompt must mention OpenCode")
	}
	if msgs[1].Get("content").String() != "hi" {
		t.Fatal("original user message must be preserved")
	}

	// Case 2: existing system message with <env> → leave untouched
	existing := []byte(`{"model":"m","messages":[{"role":"system","content":"You are an agent.\n<env>\n  Working directory: /tmp\n</env>"},{"role":"user","content":"hi"}]}`)
	out2 := EnsureOpencodeSystemPrompt(existing)
	if string(out2) != string(existing) {
		t.Fatal("payload with existing <env> system prompt must be left untouched")
	}

	// Case 3: existing system message without <env> → prepend new system before it
	noEnv := []byte(`{"model":"m","messages":[{"role":"system","content":"Custom instructions"},{"role":"user","content":"hi"}]}`)
	out3 := EnsureOpencodeSystemPrompt(noEnv)
	msgs3 := gjson.GetBytes(out3, "messages").Array()
	if len(msgs3) != 3 {
		t.Fatalf("expected 3 messages after injection, got %d", len(msgs3))
	}
	if msgs3[0].Get("role").String() != "system" || !strings.Contains(msgs3[0].Get("content").String(), "<env>") {
		t.Fatal("first message must be injected system with <env>")
	}
	if msgs3[1].Get("content").String() != "Custom instructions" {
		t.Fatal("original system message must be preserved")
	}

	// Case 4: idempotent
	out4 := EnsureOpencodeSystemPrompt(out)
	msgs4 := gjson.GetBytes(out4, "messages").Array()
	if len(msgs4) != 2 {
		t.Fatal("re-injection must be idempotent")
	}
}
