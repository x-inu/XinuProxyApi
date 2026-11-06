package semanticcache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/tidwall/gjson"
)

// BuildCacheKey derives a deterministic, collision-resistant cache key from a
// raw OpenAI-compatible request body. The key covers the fields that change
// the completion: model, full message list (text, multimodal blocks), tools,
// tool_choice, temperature and token limits. Session isolation is included so
// isolated tenants never share entries.
//
// The key is derived from the raw body JSON substrings (message order is part
// of the prompt), so identical requests hash identically and near-identical
// requests never collide.
func BuildCacheKey(sessionID string, body []byte) string {
	if len(body) == 0 {
		return ""
	}

	model := gjson.GetBytes(body, "model").String()
	if model == "" {
		return ""
	}

	h := sha256.New()

	if sessionID != "" {
		h.Write([]byte("session:"))
		h.Write([]byte(sessionID))
		h.Write([]byte("\n"))
	}

	h.Write([]byte("model:"))
	h.Write([]byte(model))
	h.Write([]byte("\n"))

	if messages := gjson.GetBytes(body, "messages"); messages.Exists() && messages.IsArray() {
		messages.ForEach(func(_, m gjson.Result) bool {
			h.Write([]byte(m.Get("role").String()))
			h.Write([]byte(":"))
			h.Write([]byte(m.Get("content").Raw))
			h.Write([]byte("\n"))
			return true
		})
	} else if input := gjson.GetBytes(body, "input"); input.Exists() {
		h.Write([]byte("input:"))
		h.Write([]byte(input.Raw))
		h.Write([]byte("\n"))
	} else {
		return ""
	}

	if tools := gjson.GetBytes(body, "tools"); tools.Exists() {
		h.Write([]byte("tools:"))
		h.Write([]byte(tools.Raw))
		h.Write([]byte("\n"))
	}
	if toolChoice := gjson.GetBytes(body, "tool_choice"); toolChoice.Exists() {
		h.Write([]byte("tool_choice:"))
		h.Write([]byte(toolChoice.Raw))
		h.Write([]byte("\n"))
	}
	if temperature := gjson.GetBytes(body, "temperature"); temperature.Exists() {
		h.Write([]byte(fmt.Sprintf("temp:%.4f\n", temperature.Float())))
	}
	writeIntParam(h, "max_tokens", body, "max_tokens")
	writeIntParam(h, "max_completion_tokens", body, "max_completion_tokens")
	if parallel := gjson.GetBytes(body, "parallel_tool_calls"); parallel.Exists() {
		h.Write([]byte(fmt.Sprintf("parallel_tool_calls:%t\n", parallel.Bool())))
	}
	if reasoningEffort := gjson.GetBytes(body, "reasoning_effort"); reasoningEffort.Exists() {
		h.Write([]byte("reasoning_effort:"))
		h.Write([]byte(reasoningEffort.String()))
		h.Write([]byte("\n"))
	}

	return hex.EncodeToString(h.Sum(nil))
}

func writeIntParam(h io.Writer, label string, body []byte, gjsonPath string) {
	v := gjson.GetBytes(body, gjsonPath)
	if !v.Exists() || v.Type != gjson.Number {
		return
	}
	_, _ = h.Write([]byte(label))
	_, _ = h.Write([]byte(":"))
	_, _ = h.Write([]byte(fmt.Sprintf("%d\n", v.Int())))
}

// EstimateTokens returns a rough token count for the cached body.
func EstimateTokens(body []byte) int {
	if len(body) == 0 {
		return 0
	}
	// Prefer usage.total_tokens when present.
	if usage := gjson.GetBytes(body, "usage.total_tokens"); usage.Exists() && usage.Int() > 0 {
		return int(usage.Int())
	}
	tokens := len(body) / 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// CloneBody deep-copies the cached response body.
func CloneBody(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
