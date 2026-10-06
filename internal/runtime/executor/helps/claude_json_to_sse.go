package helps

import (
	"bytes"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ClaudeNativeJSONToSSE converts a native (non-streaming) Claude Messages API
// JSON response into the equivalent SSE event stream that the downstream
// translators expect.
//
// Some third-party Claude-compatible upstreams have broken streaming
// implementations (e.g. they emit message_start/message_delta/message_stop but
// omit content_block_start/content_block_delta events entirely). By requesting
// stream:false from these upstreams and converting the well-formed JSON
// response to synthetic SSE, the proxy can still deliver complete content to
// clients.
//
// The synthesized SSE is intentionally minimal: message_start, one
// content_block_start + content_block_delta + content_block_stop per content
// item, then message_delta + message_stop. This is sufficient for the
// registered Claude→OpenAI and Claude→Responses translators.
func ClaudeNativeJSONToSSE(data []byte) []byte {
	root := gjson.ParseBytes(data)
	if !root.Get("type").Exists() && !root.Get("id").Exists() {
		// Not a Claude Messages response; return as-is.
		return data
	}

	var buf bytes.Buffer

	// 1. message_start — carries id, model, role, and input usage.
	messageStart := buildMessageStartEvent(root)
	buf.WriteString("event: message_start\n")
	buf.WriteString("data: ")
	buf.Write(messageStart)
	buf.WriteString("\n\n")

	// 2. content blocks
	contentBlocks := root.Get("content")
	if contentBlocks.Exists() && contentBlocks.IsArray() {
		idx := 0
		for _, block := range contentBlocks.Array() {
			blockType := block.Get("type").String()

			// content_block_start
			startEvent := buildContentBlockStartEvent(idx, block)
			buf.WriteString("event: content_block_start\n")
			buf.WriteString("data: ")
			buf.Write(startEvent)
			buf.WriteString("\n\n")

			// content_block_delta(s)
			deltaEvents := buildContentBlockDeltaEvents(idx, blockType, block)
			for _, delta := range deltaEvents {
				buf.WriteString("event: content_block_delta\n")
				buf.WriteString("data: ")
				buf.Write(delta)
				buf.WriteString("\n\n")
			}

			// content_block_stop
			stopEvent := buildContentBlockStopEvent(idx)
			buf.WriteString("event: content_block_stop\n")
			buf.WriteString("data: ")
			buf.Write(stopEvent)
			buf.WriteString("\n\n")

			idx++
		}
	}

	// 3. message_delta — carries stop_reason and output usage.
	messageDelta := buildMessageDeltaEvent(root)
	buf.WriteString("event: message_delta\n")
	buf.WriteString("data: ")
	buf.Write(messageDelta)
	buf.WriteString("\n\n")

	// 4. message_stop
	buf.WriteString("event: message_stop\n")
	buf.WriteString("data: {\"type\":\"message_stop\"}\n\n")

	return buf.Bytes()
}

func buildMessageStartEvent(root gjson.Result) []byte {
	// Build a message object with input-side usage only (output_tokens: 0).
	msg := []byte(`{"type":"message_start","message":{}}`)

	// Copy top-level message fields.
	for _, key := range []string{"id", "model", "role", "type"} {
		if v := root.Get(key); v.Exists() {
			if key == "type" {
				msg, _ = sjson.SetBytes(msg, "message.type", v.Value())
			} else {
				msg, _ = sjson.SetBytes(msg, "message."+key, v.Value())
			}
		}
	}
	// Empty content array (content is delivered through content_block events).
	msg, _ = sjson.SetRawBytes(msg, "message.content", []byte("[]"))

	// Usage: only input_tokens and cache fields from the original response.
	usage := root.Get("usage")
	if usage.Exists() {
		usageOut := []byte(`{}`)
		if v := usage.Get("input_tokens"); v.Exists() {
			usageOut, _ = sjson.SetBytes(usageOut, "input_tokens", v.Int())
		}
		if v := usage.Get("cache_creation_input_tokens"); v.Exists() {
			usageOut, _ = sjson.SetBytes(usageOut, "cache_creation_input_tokens", v.Int())
		}
		if v := usage.Get("cache_read_input_tokens"); v.Exists() {
			usageOut, _ = sjson.SetBytes(usageOut, "cache_read_input_tokens", v.Int())
		}
		usageOut, _ = sjson.SetBytes(usageOut, "output_tokens", int64(0))
		msg, _ = sjson.SetRawBytes(msg, "message.usage", usageOut)
	}

	// Set stop_reason and stop_sequence to null in the message_start.
	msg, _ = sjson.SetBytes(msg, "message.stop_reason", nil)
	msg, _ = sjson.SetBytes(msg, "message.stop_sequence", nil)

	return msg
}

func buildContentBlockStartEvent(index int, block gjson.Result) []byte {
	event := []byte(`{"type":"content_block_start"}`)
	event, _ = sjson.SetBytes(event, "index", index)

	blockType := block.Get("type").String()
	switch blockType {
	case "text":
		event, _ = sjson.SetRawBytes(event, "content_block", []byte(`{"type":"text","text":""}`))
	case "thinking":
		event, _ = sjson.SetRawBytes(event, "content_block", []byte(`{"type":"thinking","thinking":""}`))
	case "tool_use":
		cb := []byte(`{"type":"tool_use","input":{}}`)
		if id := block.Get("id"); id.Exists() {
			cb, _ = sjson.SetBytes(cb, "id", id.String())
		}
		if name := block.Get("name"); name.Exists() {
			cb, _ = sjson.SetBytes(cb, "name", name.String())
		}
		event, _ = sjson.SetRawBytes(event, "content_block", cb)
	default:
		// Unknown block type; pass through a minimal block.
		cb, _ := sjson.SetBytes([]byte(`{}`), "type", blockType)
		event, _ = sjson.SetRawBytes(event, "content_block", cb)
	}
	return event
}

func buildContentBlockDeltaEvents(index int, blockType string, block gjson.Result) [][]byte {
	var events [][]byte
	switch blockType {
	case "text":
		text := block.Get("text").String()
		if text == "" {
			return events
		}
		delta := []byte(`{"type":"content_block_delta"}`)
		delta, _ = sjson.SetBytes(delta, "index", index)
		delta, _ = sjson.SetRawBytes(delta, "delta", []byte(`{"type":"text_delta"}`))
		delta, _ = sjson.SetBytes(delta, "delta.text", text)
		events = append(events, delta)
	case "thinking":
		thinking := block.Get("thinking").String()
		if thinking == "" {
			return events
		}
		delta := []byte(`{"type":"content_block_delta"}`)
		delta, _ = sjson.SetBytes(delta, "index", index)
		delta, _ = sjson.SetRawBytes(delta, "delta", []byte(`{"type":"thinking_delta"}`))
		delta, _ = sjson.SetBytes(delta, "delta.thinking", thinking)
		events = append(events, delta)
	case "tool_use":
		input := block.Get("input").Raw
		if input == "" || input == "{}" {
			input = "{}"
		}
		delta := []byte(`{"type":"content_block_delta"}`)
		delta, _ = sjson.SetBytes(delta, "index", index)
		delta, _ = sjson.SetRawBytes(delta, "delta", []byte(`{"type":"input_json_delta"}`))
		delta, _ = sjson.SetBytes(delta, "delta.partial_json", input)
		events = append(events, delta)
	}
	return events
}

func buildContentBlockStopEvent(index int) []byte {
	event := []byte(`{"type":"content_block_stop"}`)
	event, _ = sjson.SetBytes(event, "index", index)
	return event
}

func buildMessageDeltaEvent(root gjson.Result) []byte {
	event := []byte(`{"type":"message_delta","delta":{},"usage":{}}`)

	// stop_reason
	if sr := root.Get("stop_reason"); sr.Exists() {
		event, _ = sjson.SetBytes(event, "delta.stop_reason", sr.Value())
	}
	// stop_sequence
	if ss := root.Get("stop_sequence"); ss.Exists() {
		event, _ = sjson.SetBytes(event, "delta.stop_sequence", ss.Value())
	}

	// Output usage in message_delta.
	usage := root.Get("usage")
	if usage.Exists() {
		if v := usage.Get("output_tokens"); v.Exists() {
			event, _ = sjson.SetBytes(event, "usage.output_tokens", v.Int())
		}
		// Carry input usage fields too so that usage merging works correctly
		// when the non-stream translator processes message_delta usage.
		if v := usage.Get("input_tokens"); v.Exists() {
			event, _ = sjson.SetBytes(event, "usage.input_tokens", v.Int())
		}
		if v := usage.Get("cache_creation_input_tokens"); v.Exists() {
			event, _ = sjson.SetBytes(event, "usage.cache_creation_input_tokens", v.Int())
		}
		if v := usage.Get("cache_read_input_tokens"); v.Exists() {
			event, _ = sjson.SetBytes(event, "usage.cache_read_input_tokens", v.Int())
		}
	}

	return event
}
