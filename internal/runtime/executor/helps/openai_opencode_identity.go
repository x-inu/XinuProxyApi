package helps

import (
	"encoding/hex"
	"hash/fnv"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/tidwall/gjson"
)

// OpenCode free-tier identity synthesis. Mirrors the behavior of OmniRoute's
// opencode executor: keyless requests carry OpenCode CLI identity headers so
// the upstream free tier accepts them, with a per-request UUID and a
// body-derived stable session id. Explicit custom headers configured on the
// provider still win because they are applied after this helper.
const (
	opencodeUserAgent       = "opencode"
	opencodeClientDefault   = "desktop"
	opencodeProjectDefault  = "global"
	opencodeRequestHeader   = "x-opencode-request"
	opencodeSessionHeader   = "x-opencode-session"
	opencodeClientHeader    = "x-opencode-client"
	opencodeProjectHeader   = "x-opencode-project"
	opencodeFreeModelSuffix = "-free"
)

// OpencodeIdentityEnabled reports whether the resolved OpenAI compatibility
// entry requests OpenCode free-tier identity synthesis.
func OpencodeIdentityEnabled(compat *config.OpenAICompatibility) bool {
	return compat != nil && compat.SynthesizeOpencodeIdentity != nil && *compat.SynthesizeOpencodeIdentity
}

// IsOpencodeFreeModel reports whether a model id belongs to the free tier.
// The upstream convention is a "-free" suffix; anything else is premium.
func IsOpencodeFreeModel(model string) bool {
	return strings.HasSuffix(strings.TrimSpace(model), opencodeFreeModelSuffix)
}

// OpencodeFreeTierRejection returns a rejection message when a keyless request
// targets a premium model on an identity-synthesized entry. Keyed entries and
// free-tier models pass through.
func OpencodeFreeTierRejection(compat *config.OpenAICompatibility, upstreamModel, apiKey string) (string, bool) {
	if !OpencodeIdentityEnabled(compat) {
		return "", false
	}
	if strings.TrimSpace(apiKey) != "" {
		return "", false
	}
	if IsOpencodeFreeModel(upstreamModel) {
		return "", false
	}
	return "model " + strings.TrimSpace(upstreamModel) + " requires an opencode API key on the free tier", true
}

// ApplyOpencodeIdentity synthesizes OpenCode CLI identity headers on the
// outgoing upstream request. Existing header values are preserved except for
// x-opencode-request, which is always refreshed with a new UUID. The payload
// is only used to derive a stable session id and may be nil.
func ApplyOpencodeIdentity(r *http.Request, compat *config.OpenAICompatibility, payload []byte) {
	if !OpencodeIdentityEnabled(compat) || r == nil {
		return
	}
	r.Header.Set("User-Agent", opencodeUserAgent)
	if r.Header.Get(opencodeClientHeader) == "" {
		r.Header.Set(opencodeClientHeader, opencodeClientDefault)
	}
	if r.Header.Get(opencodeProjectHeader) == "" {
		r.Header.Set(opencodeProjectHeader, opencodeProjectDefault)
	}
	r.Header.Set(opencodeRequestHeader, uuid.NewString())
	if r.Header.Get(opencodeSessionHeader) == "" {
		if sessionID := OpencodeSessionID(payload); sessionID != "" {
			r.Header.Set(opencodeSessionHeader, sessionID)
		}
	}
}

// OpencodeSessionID derives a stable session fingerprint from the request body
// (model, system prompt, first user message, tool names) so requests from one
// conversation share an upstream session. Returns "" when the body carries no
// usable signal.
func OpencodeSessionID(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	var parts []string
	if model := strings.TrimSpace(gjson.GetBytes(payload, "model").String()); model != "" {
		parts = append(parts, "model:"+model)
	}
	if system := opencodeTextContent(gjson.GetBytes(payload, "system")); system != "" {
		parts = append(parts, "sys:"+shortHash(system))
	}
	messages := gjson.GetBytes(payload, "messages")
	if messages.IsArray() {
		for _, msg := range messages.Array() {
			if strings.TrimSpace(msg.Get("role").String()) != "user" {
				continue
			}
			if text := opencodeTextContent(msg.Get("content")); text != "" {
				parts = append(parts, "user0:"+shortHash(text))
				break
			}
		}
	}
	var tools []string
	for _, tool := range gjson.GetBytes(payload, "tools").Array() {
		name := strings.TrimSpace(tool.Get("function.name").String())
		if name == "" {
			name = strings.TrimSpace(tool.Get("name").String())
		}
		if name != "" {
			tools = append(tools, name)
		}
	}
	if len(tools) > 0 {
		sort.Strings(tools)
		parts = append(parts, "tools:"+shortHash(strings.Join(tools, ",")))
	}
	if len(parts) == 0 {
		return ""
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h.Sum(nil))
}

// opencodeTextContent extracts plain text from an OpenAI content value, which
// may be a string or an array of parts with type "text".
func opencodeTextContent(node gjson.Result) string {
	if node.Type == gjson.String {
		return firstN(node.String(), 256)
	}
	if node.IsArray() {
		var sb strings.Builder
		for _, part := range node.Array() {
			if strings.TrimSpace(part.Get("type").String()) != "text" {
				continue
			}
			text := part.Get("text").String()
			if text == "" {
				continue
			}
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(text)
			if sb.Len() >= 256 {
				break
			}
		}
		return firstN(sb.String(), 256)
	}
	return ""
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func shortHash(s string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:4])
}
