package helps

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// OpenCode free-tier identity synthesis. Mirrors the traffic shape of the
// genuine OpenCode CLI (reverse-engineered from the open-source client):
// keyless free-tier requests carry a versioned CLI User-Agent, a minted
// ses_ session id, and read+shell tool definitions so the upstream free
// tier accepts them, plus streaming (locked models are served on SSE only).
// Explicit custom headers configured on the provider still win because they
// are applied after this helper.
const (
	opencodeUserAgent       = "opencode/latest/2.0.25/cli"
	opencodeClientDefault   = "cli"
	opencodeProjectDefault  = "global"
	opencodeRequestHeader   = "x-opencode-request"
	opencodeSessionHeader   = "x-opencode-session"
	opencodeClientHeader    = "x-opencode-client"
	opencodeProjectHeader   = "x-opencode-project"
	opencodeFreeModelSuffix = "-free"

	// opencodeLegacyOpenModel is served upstream without the recipe
	// (no tools, no streaming required). Keep its legacy shape untouched.
	opencodeLegacyOpenModel = "space-bunny-free"

	opencodeSessionPrefix = "ses_"
	opencodeSessionTail   = 14
	opencodeSessionAlpha  = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
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

// IsOpencodeLegacyOpenModel reports whether a model id is served upstream
// without the unlock recipe (plain keyless requests already succeed).
func IsOpencodeLegacyOpenModel(model string) bool {
	trimmed := strings.TrimSpace(model)
	return trimmed == opencodeLegacyOpenModel || strings.HasSuffix(trimmed, "/"+opencodeLegacyOpenModel)
}

// NeedsOpencodeFullRecipe reports whether a keyless free-tier request needs
// the full unlock recipe (versioned identity + minted session + marker
// tools + forced streaming). Keyed entries and the legacy-open model pass
// through untouched.
func NeedsOpencodeFullRecipe(compat *config.OpenAICompatibility, upstreamModel, apiKey string) bool {
	if !OpencodeIdentityEnabled(compat) {
		return false
	}
	if strings.TrimSpace(apiKey) != "" {
		return false
	}
	if !IsOpencodeFreeModel(upstreamModel) {
		return false
	}
	return !IsOpencodeLegacyOpenModel(upstreamModel)
}

// OpencodeUpstreamModel extracts the upstream model id from a translated
// OpenAI chat payload.
func OpencodeUpstreamModel(payload []byte) string {
	return strings.TrimSpace(gjson.GetBytes(payload, "model").String())
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
// is retained for signature compatibility and currently unused.
func ApplyOpencodeIdentity(r *http.Request, compat *config.OpenAICompatibility, _ []byte) {
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
		r.Header.Set(opencodeSessionHeader, MintOpencodeSessionID())
	}
}

var (
	opencodeSessionMu      sync.Mutex
	opencodeSessionLastMs  int64
	opencodeSessionCounter uint32
)

// MintOpencodeSessionID mints a fresh ses_ session id with the same shape as
// the genuine CLI (12 hex chars encoding ~(now_ms*4096+counter) in 48 bits,
// plus 14 alphanumeric chars). The upstream free tier validates the embedded
// timestamp window, so ids must be minted fresh, never random.
func MintOpencodeSessionID() string {
	now := time.Now().UnixMilli()
	opencodeSessionMu.Lock()
	if now != opencodeSessionLastMs {
		opencodeSessionLastMs = now
		opencodeSessionCounter = 0
	}
	opencodeSessionCounter++
	counter := opencodeSessionCounter & 0xFFF
	opencodeSessionMu.Unlock()

	n := new(big.Int).SetInt64(now)
	n.Lsh(n, 12)
	n.Or(n, big.NewInt(int64(counter)))
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 48), big.NewInt(1))
	head := new(big.Int).AndNot(mask, n)
	prefix := fmt.Sprintf("%012x", head)

	tail := make([]byte, opencodeSessionTail)
	var buf [1]byte
	for i := range tail {
		if _, err := rand.Read(buf[:]); err != nil {
			tail[i] = opencodeSessionAlpha[i%len(opencodeSessionAlpha)]
			continue
		}
		tail[i] = opencodeSessionAlpha[int(buf[0])%len(opencodeSessionAlpha)]
	}
	return opencodeSessionPrefix + prefix + string(tail)
}

// opencodeSystemPrompt is a minimal system prompt carrying the same structural
// markers the OpenCode CLI embeds: an agent preamble, a # Harness section, and
// an <env> block with at least a Working directory. The upstream free-tier
// gate inspects these markers when deciding whether to serve a request.
const opencodeSystemPrompt = "You are an AI agent running in OpenCode, a coding agent harness. " +
	"Help the user accomplish their goals using the tools you have available.\n\n" +
	"# Harness\n" +
	"- Responses are rendered as GitHub-flavored Markdown.\n" +
	"- `<system-reminder>` blocks are harness instructions, not user-authored content. Read and follow them.\n" +
	"- Prefer parallelizing independent tool calls.\n" +
	"- Prefer dedicated tools over shell commands; fall back to the shell when a tool cannot do what you need.\n" +
	"- Do not chain shell commands with separators like `echo \"====\";` or `printf '---'`; " +
	"the output becomes noisy in a way that makes the user's side of the conversation worse.\n" +
	"- Use the write tool to create files or completely replace their content. " +
	"Prefer using the edit tool for targeted changes.\n" +
	"- Use the edit tool for targeted changes to existing text files. " +
	"It replaces the exact text in `oldString` with `newString`.\n\n" +
	"# Communication\n" +
	"- Use clear file paths when referring to files.\n" +
	"- Keep responses clear and concise, and avoid unnecessary technical jargon.\n\n" +
	"# Working in codebases\n" +
	"- Keep changes consistent with the structure, naming, style, and patterns of the surrounding code.\n" +
	"- Treat unfamiliar files or changes as potential user work and investigate before deleting or overwriting them.\n\n" +
	"# Your Model\n" +
	"- Provider ID: opencode\n\n" +
	"<env>\n" +
	"  Working directory: /home/user\n" +
	"  Platform: linux\n" +
	"</env>\n"

// EnsureOpencodeSystemPrompt prepends the CLI-shaped system prompt when the
// payload's first message is not already a system message, or when the
// existing system message does not contain the structural <env> marker.
func EnsureOpencodeSystemPrompt(payload []byte) []byte {
	msgs := gjson.GetBytes(payload, "messages")
	if !msgs.Exists() || !msgs.IsArray() {
		return payload
	}
	arr := msgs.Array()
	if len(arr) > 0 {
		first := arr[0]
		if first.Get("role").String() == "system" && strings.Contains(first.Get("content").String(), "<env>") {
			return payload
		}
	}
	sysMsg, err := json.Marshal(map[string]string{"role": "system", "content": opencodeSystemPrompt})
	if err != nil {
		return payload
	}
	raw := gjson.GetBytes(payload, "messages").Raw
	if len(raw) < 2 {
		return payload
	}
	newMsgs := "[" + string(sysMsg) + "," + raw[1:]
	updated, err := sjson.SetRawBytes(payload, "messages", []byte(newMsgs))
	if err != nil {
		return payload
	}
	return updated
}

// Marker tool definitions injected when a keyless free-tier request carries
// no read+shell tools. The upstream gate only inspects tool names.
const (
	opencodeMarkerReadTool  = `{"type":"function","function":{"name":"read","description":"read a file","parameters":{"type":"object","properties":{"p":{"type":"string"}}}}}`
	opencodeMarkerShellTool = `{"type":"function","function":{"name":"shell","description":"run a command","parameters":{"type":"object","properties":{"q":{"type":"string"}}}}}`
)

// EnsureOpencodeMarkerTools appends read+shell marker tool definitions to an
// OpenAI chat payload when either name is missing. Payloads that already
// carry both, or whose tools field is not an array, are returned untouched.
func EnsureOpencodeMarkerTools(payload []byte) []byte {
	tools := gjson.GetBytes(payload, "tools")
	if tools.Exists() && !tools.IsArray() {
		return payload
	}
	var hasRead, hasShell bool
	for _, tool := range tools.Array() {
		name := strings.TrimSpace(tool.Get("function.name").String())
		if name == "" {
			name = strings.TrimSpace(tool.Get("name").String())
		}
		switch name {
		case "read":
			hasRead = true
		case "shell":
			hasShell = true
		}
	}
	if hasRead && hasShell {
		return payload
	}
	out := payload
	if !hasRead {
		updated, err := sjson.SetRawBytes(out, "tools.-1", []byte(opencodeMarkerReadTool))
		if err != nil {
			return payload
		}
		out = updated
	}
	if !hasShell {
		updated, err := sjson.SetRawBytes(out, "tools.-1", []byte(opencodeMarkerShellTool))
		if err != nil {
			return payload
		}
		out = updated
	}
	return out
}

// AssembleOpencodeStreamCompletion folds SSE chat-completion chunks into a
// single non-streaming completion body for clients that did not request a
// stream, preserving text, reasoning fallback, tool calls, finish reasons
// and usage. Bodies without SSE frames are returned untouched.
func AssembleOpencodeStreamCompletion(body []byte, model string) []byte {
	type assembledTool struct {
		id, callType, name string
		args               strings.Builder
	}
	type assembledChoice struct {
		content, reasoning strings.Builder
		finish             string
		tools              map[int]*assembledTool
		toolOrder          []int
	}
	choices := map[int]*assembledChoice{}
	var choiceOrder []int
	choiceState := func(index int) *assembledChoice {
		state, ok := choices[index]
		if !ok {
			state = &assembledChoice{tools: map[int]*assembledTool{}}
			choices[index] = state
			choiceOrder = append(choiceOrder, index)
		}
		return state
	}
	var frames int
	var usageRaw string
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		frames++
		chunk := gjson.Parse(data)
		for _, choice := range chunk.Get("choices").Array() {
			state := choiceState(int(choice.Get("index").Int()))
			if text := choice.Get("delta.content").String(); text != "" {
				state.content.WriteString(text)
			}
			if text := choice.Get("delta.reasoning_content").String(); text != "" {
				state.reasoning.WriteString(text)
			}
			for _, call := range choice.Get("delta.tool_calls").Array() {
				index := int(call.Get("index").Int())
				tool, ok := state.tools[index]
				if !ok {
					tool = &assembledTool{}
					state.tools[index] = tool
					state.toolOrder = append(state.toolOrder, index)
				}
				if id := strings.TrimSpace(call.Get("id").String()); id != "" {
					tool.id = id
				}
				callType := strings.TrimSpace(call.Get("type").String())
				if callType == "" {
					callType = "function"
				}
				tool.callType = callType
				if name := strings.TrimSpace(call.Get("function.name").String()); name != "" {
					tool.name = name
				}
				tool.args.WriteString(call.Get("function.arguments").String())
			}
			if reason := strings.TrimSpace(choice.Get("finish_reason").String()); reason != "" && reason != "null" {
				state.finish = reason
			}
		}
		if usage := chunk.Get("usage"); usage.Exists() && usage.IsObject() {
			usageRaw = usage.Raw
		}
	}
	if frames == 0 {
		return body
	}
	out := []byte(`{"object":"chat.completion"}`)
	var err error
	if out, err = sjson.SetBytes(out, "id", "chatcmpl-opencode-"+uuid.NewString()[:8]); err != nil {
		return body
	}
	if out, err = sjson.SetBytes(out, "created", time.Now().Unix()); err != nil {
		return body
	}
	if out, err = sjson.SetBytes(out, "model", model); err != nil {
		return body
	}
	var choicesJSON strings.Builder
	choicesJSON.WriteByte('[')
	for i, index := range choiceOrder {
		state := choices[index]
		if i > 0 {
			choicesJSON.WriteByte(',')
		}
		text := state.content.String()
		if text == "" {
			text = state.reasoning.String()
		}
		message := []byte(`{"role":"assistant"}`)
		quoted, err := json.Marshal(text)
		if err != nil {
			return body
		}
		if message, err = sjson.SetRawBytes(message, "content", quoted); err != nil {
			return body
		}
		if len(state.toolOrder) > 0 {
			var callsJSON strings.Builder
			callsJSON.WriteByte('[')
			for j, toolIndex := range state.toolOrder {
				tool := state.tools[toolIndex]
				if j > 0 {
					callsJSON.WriteByte(',')
				}
				args, err := json.Marshal(tool.args.String())
				if err != nil {
					return body
				}
				call, err := sjson.SetBytes([]byte(`{"index":0,"type":"function"}`), "index", toolIndex)
				if err != nil {
					return body
				}
				if tool.id != "" {
					if call, err = sjson.SetBytes(call, "id", tool.id); err != nil {
						return body
					}
				}
				if tool.callType != "" {
					if call, err = sjson.SetBytes(call, "type", tool.callType); err != nil {
						return body
					}
				}
				fn, err := sjson.SetBytes([]byte(`{}`), "name", tool.name)
				if err != nil {
					return body
				}
				if fn, err = sjson.SetRawBytes(fn, "arguments", args); err != nil {
					return body
				}
				if call, err = sjson.SetRawBytes(call, "function", fn); err != nil {
					return body
				}
				callsJSON.Write(call)
			}
			callsJSON.WriteByte(']')
			if message, err = sjson.SetRawBytes(message, "tool_calls", []byte(callsJSON.String())); err != nil {
				return body
			}
		}
		choiceJSON := []byte(`{"index":0}`)
		if choiceJSON, err = sjson.SetBytes(choiceJSON, "index", index); err != nil {
			return body
		}
		if choiceJSON, err = sjson.SetRawBytes(choiceJSON, "message", message); err != nil {
			return body
		}
		finish := state.finish
		if finish == "" {
			finish = "stop"
		}
		if choiceJSON, err = sjson.SetBytes(choiceJSON, "finish_reason", finish); err != nil {
			return body
		}
		choicesJSON.Write(choiceJSON)
	}
	choicesJSON.WriteByte(']')
	if out, err = sjson.SetRawBytes(out, "choices", []byte(choicesJSON.String())); err != nil {
		return body
	}
	if usageRaw != "" {
		if out, err = sjson.SetRawBytes(out, "usage", []byte(usageRaw)); err != nil {
			return body
		}
	}
	return out
}
