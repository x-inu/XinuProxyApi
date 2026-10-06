package helps

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestClaudeNativeJSONToSSE_TextContent(t *testing.T) {
	input := []byte(`{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"model": "claude-opus-4-8",
		"content": [{"type": "text", "text": "TEST OK"}],
		"stop_reason": "end_turn",
		"stop_sequence": null,
		"usage": {
			"input_tokens": 10375,
			"output_tokens": 7,
			"cache_creation_input_tokens": 10373
		}
	}`)

	result := ClaudeNativeJSONToSSE(input)
	resultStr := string(result)

	// Must contain all required SSE event types.
	for _, required := range []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		"event: content_block_stop",
		"event: message_delta",
		"event: message_stop",
	} {
		if !strings.Contains(resultStr, required) {
			t.Fatalf("missing %q in SSE output:\n%s", required, resultStr)
		}
	}

	// Parse each data: line and verify.
	lines := strings.Split(resultStr, "\n")
	var dataLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}

	if len(dataLines) < 5 {
		t.Fatalf("expected at least 5 data lines, got %d: %v", len(dataLines), dataLines)
	}

	// message_start
	ms := gjson.Parse(dataLines[0])
	if ms.Get("type").String() != "message_start" {
		t.Fatalf("expected message_start, got %s", ms.Get("type").String())
	}
	if ms.Get("message.id").String() != "msg_123" {
		t.Fatalf("expected id msg_123, got %s", ms.Get("message.id").String())
	}
	if ms.Get("message.model").String() != "claude-opus-4-8" {
		t.Fatalf("expected model claude-opus-4-8, got %s", ms.Get("message.model").String())
	}

	// content_block_start
	cbs := gjson.Parse(dataLines[1])
	if cbs.Get("type").String() != "content_block_start" {
		t.Fatalf("expected content_block_start, got %s", cbs.Get("type").String())
	}
	if cbs.Get("content_block.type").String() != "text" {
		t.Fatalf("expected text block type, got %s", cbs.Get("content_block.type").String())
	}

	// content_block_delta
	cbd := gjson.Parse(dataLines[2])
	if cbd.Get("type").String() != "content_block_delta" {
		t.Fatalf("expected content_block_delta, got %s", cbd.Get("type").String())
	}
	if cbd.Get("delta.type").String() != "text_delta" {
		t.Fatalf("expected text_delta, got %s", cbd.Get("delta.type").String())
	}
	if cbd.Get("delta.text").String() != "TEST OK" {
		t.Fatalf("expected text 'TEST OK', got %q", cbd.Get("delta.text").String())
	}

	// message_delta
	md := gjson.Parse(dataLines[4])
	if md.Get("type").String() != "message_delta" {
		t.Fatalf("expected message_delta, got %s", md.Get("type").String())
	}
	if md.Get("delta.stop_reason").String() != "end_turn" {
		t.Fatalf("expected stop_reason end_turn, got %s", md.Get("delta.stop_reason").String())
	}
	if md.Get("usage.output_tokens").Int() != 7 {
		t.Fatalf("expected output_tokens 7, got %d", md.Get("usage.output_tokens").Int())
	}
}

func TestClaudeNativeJSONToSSE_ToolUseContent(t *testing.T) {
	input := []byte(`{
		"id": "msg_456",
		"type": "message",
		"role": "assistant",
		"model": "claude-opus-4-8",
		"content": [
			{"type": "text", "text": "I'll look that up."},
			{"type": "tool_use", "id": "toolu_1", "name": "search", "input": {"query": "test"}}
		],
		"stop_reason": "tool_use",
		"stop_sequence": null,
		"usage": {"input_tokens": 100, "output_tokens": 50}
	}`)

	result := ClaudeNativeJSONToSSE(input)
	resultStr := string(result)

	// Should have 2 content_block_start events.
	if count := strings.Count(resultStr, "event: content_block_start"); count != 2 {
		t.Fatalf("expected 2 content_block_start events, got %d", count)
	}
	if count := strings.Count(resultStr, "event: content_block_stop"); count != 2 {
		t.Fatalf("expected 2 content_block_stop events, got %d", count)
	}

	// Verify tool_use block carries id and name.
	if !strings.Contains(resultStr, `"name":"search"`) {
		t.Fatal("missing tool name in SSE output")
	}
	if !strings.Contains(resultStr, `"id":"toolu_1"`) {
		t.Fatal("missing tool id in SSE output")
	}
}

func TestClaudeNativeJSONToSSE_EmptyContent(t *testing.T) {
	input := []byte(`{
		"id": "msg_789",
		"type": "message",
		"role": "assistant",
		"model": "claude-opus-4-8",
		"content": [],
		"stop_reason": "end_turn",
		"stop_sequence": null,
		"usage": {"input_tokens": 10, "output_tokens": 0}
	}`)

	result := ClaudeNativeJSONToSSE(input)
	resultStr := string(result)

	// Must still have message_start and message_delta/stop, but no content blocks.
	if !strings.Contains(resultStr, "event: message_start") {
		t.Fatal("missing message_start")
	}
	if !strings.Contains(resultStr, "event: message_delta") {
		t.Fatal("missing message_delta")
	}
	if !strings.Contains(resultStr, "event: message_stop") {
		t.Fatal("missing message_stop")
	}
	if strings.Contains(resultStr, "event: content_block_start") {
		t.Fatal("unexpected content_block_start for empty content")
	}
}

func TestClaudeNativeJSONToSSE_NotClaudeResponse(t *testing.T) {
	input := []byte(`{"error": "something went wrong"}`)
	result := ClaudeNativeJSONToSSE(input)
	if string(result) != string(input) {
		t.Fatalf("expected passthrough for non-Claude response, got %s", string(result))
	}
}

func TestClaudeNativeJSONToSSE_ThinkingContent(t *testing.T) {
	input := []byte(`{
		"id": "msg_think",
		"type": "message",
		"role": "assistant",
		"model": "claude-opus-4-8",
		"content": [
			{"type": "thinking", "thinking": "Let me reason..."},
			{"type": "text", "text": "The answer is 4."}
		],
		"stop_reason": "end_turn",
		"stop_sequence": null,
		"usage": {"input_tokens": 20, "output_tokens": 15}
	}`)

	result := ClaudeNativeJSONToSSE(input)
	resultStr := string(result)

	if count := strings.Count(resultStr, "event: content_block_start"); count != 2 {
		t.Fatalf("expected 2 content_block_start events, got %d", count)
	}
	if !strings.Contains(resultStr, `"thinking_delta"`) {
		t.Fatal("missing thinking_delta in SSE output")
	}
	if !strings.Contains(resultStr, `"text_delta"`) {
		t.Fatal("missing text_delta in SSE output")
	}
}
