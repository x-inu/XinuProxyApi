package helps

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestOpencodeResponsesProtocolModel(t *testing.T) {
	for _, model := range []string{
		"muse-spark-1.3-contributor-free",
		"muse-spark-1.2-contributor-free",
		"oc/muse-spark-1.3-contributor-free",
	} {
		if !OpencodeResponsesProtocolModel(model) {
			t.Fatalf("model %q must use the responses protocol", model)
		}
	}
	for _, model := range []string{
		"mimo-v2.6-flash-free",
		"space-bunny-free",
		"big-pickle",
		"",
	} {
		if OpencodeResponsesProtocolModel(model) {
			t.Fatalf("model %q must not use the responses protocol", model)
		}
	}
}

func TestBuildOpencodeResponsesRequest(t *testing.T) {
	chat := []byte(`{"model":"muse-spark-1.3-contributor-free","messages":[{"role":"system","content":"Be brief."},{"role":"user","content":"hi"},{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"shell","arguments":"{\"q\": \"ls\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"out"}],"tools":[{"type":"function","function":{"name":"read","description":"read a file","parameters":{"type":"object","properties":{"p":{"type":"string"}}}}}],"tool_choice":"auto","max_completion_tokens":100,"temperature":0.5,"stream":false}`)
	built := BuildOpencodeResponsesRequest(chat)
	if gjson.GetBytes(built, "model").String() != "muse-spark-1.3-contributor-free" {
		t.Fatalf("model must pass through, got %s", built)
	}
	if gjson.GetBytes(built, "instructions").String() != "Be brief." {
		t.Fatalf("system must become instructions, got %s", built)
	}
	if gjson.GetBytes(built, "stream").Bool() != true {
		t.Fatal("streaming must be forced")
	}
	if gjson.GetBytes(built, "max_output_tokens").Int() != 100 {
		t.Fatal("max_completion_tokens must map to max_output_tokens")
	}
	if gjson.GetBytes(built, "temperature").Float() != 0.5 {
		t.Fatal("temperature must pass through")
	}
	input := gjson.GetBytes(built, "input").Array()
	if len(input) != 3 {
		t.Fatalf("input must hold user, function_call and function_call_output items, got %d: %s", len(input), gjson.GetBytes(built, "input").Raw)
	}
	if input[1].Get("type").String() != "function_call" || input[1].Get("name").String() != "shell" {
		t.Fatalf("assistant tool call must become a function_call item, got %s", input[1].Raw)
	}
	if input[1].Get("arguments").Type != gjson.String {
		t.Fatalf("function_call arguments must stay a JSON-encoded string, got %s", input[1].Get("arguments").Raw)
	}
	if input[2].Get("type").String() != "function_call_output" || input[2].Get("call_id").String() != "call_1" {
		t.Fatalf("tool message must become function_call_output, got %s", input[2].Raw)
	}
	tools := gjson.GetBytes(built, "tools").Array()
	if len(tools) != 1 || tools[0].Get("name").String() != "read" {
		t.Fatalf("chat tool name must be hoisted, got %s", gjson.GetBytes(built, "tools").Raw)
	}
	if gjson.GetBytes(built, "tool_choice").String() != "auto" {
		t.Fatalf("tool_choice auto must pass through, got %s", gjson.GetBytes(built, "tool_choice").Raw)
	}

	broken := BuildOpencodeResponsesRequest([]byte(`{"model":`))
	if string(broken) != `{"model":` {
		t.Fatal("unbuildable payloads must pass through untouched")
	}
}

func TestEnsureOpencodeResponsesMarkerTools(t *testing.T) {
	bare := []byte(`{"model":"muse-spark-1.3-contributor-free","input":[]}`)
	filled := EnsureOpencodeResponsesMarkerTools(bare)
	tools := gjson.GetBytes(filled, "tools").Array()
	if len(tools) != 2 || tools[0].Get("name").String() != "read" || tools[1].Get("name").String() != "shell" {
		t.Fatalf("bare request must gain read+shell markers, got %s", gjson.GetBytes(filled, "tools").String())
	}
	if tools[0].Get("function.name").Exists() {
		t.Fatal("markers must use responses shape, not nested function shape")
	}
	if out := EnsureOpencodeResponsesMarkerTools(filled); len(gjson.GetBytes(out, "tools").Array()) != 2 {
		t.Fatal("marker injection must be idempotent")
	}
	weird := []byte(`{"model":"m","tools":{}}`)
	if out := EnsureOpencodeResponsesMarkerTools(weird); string(out) != string(weird) {
		t.Fatal("non-array tools must be left untouched")
	}
}

// Real event payloads captured from the genuine CLI against zen.
const opencodeResponsesTextDelta = `{"type":"response.output_text.delta","sequence_number":6,"output_index":1,"content_index":0,"item_id":"rs_01","delta":"I'll list t"}`

const opencodeResponsesCallAdded = `{"type":"response.output_item.added","sequence_number":10,"output_index":2,"item":{"id":"fc_01","type":"function_call","status":"in_progress","name":"shell","call_id":"call_01","arguments":""}}`

const opencodeResponsesArgsDelta = `{"type":"response.function_call_arguments.delta","sequence_number":11,"output_index":2,"item_id":"fc_01","delta":"{\"q\":\"ls -la\"}"}`

const opencodeResponsesCompleted = `{"type":"response.completed","sequence_number":14,"response":{"id":"resp_01","status":"completed","model":"muse-spark-1.3-contributor-free","output":[{"id":"fc_01","type":"function_call","status":"completed","name":"shell","call_id":"call_01","arguments":"{\"q\":\"ls -la\"}"}],"usage":{"input_tokens":584,"output_tokens":537,"total_tokens":1121}}}`

func TestTranslateOpencodeResponsesStream(t *testing.T) {
	state := &opencodeResponsesChatState{}
	frames := TranslateOpencodeResponsesStream("muse-spark-1.3-contributor-free", state, "response.output_text.delta", opencodeResponsesTextDelta)
	if len(frames) != 1 {
		t.Fatalf("text delta must yield one chat chunk, got %d", len(frames))
	}
	if got := gjson.GetBytes(frames[0], "choices.0.delta.content").String(); got != "I'll list t" {
		t.Fatalf("chat delta content = %q", got)
	}
	if got := gjson.GetBytes(frames[0], "choices.0.delta.role").String(); got != "assistant" {
		t.Fatalf("first chunk must carry the role, got %q", got)
	}

	frames = TranslateOpencodeResponsesStream("muse-spark-1.3-contributor-free", state, "response.output_item.added", opencodeResponsesCallAdded)
	if len(frames) != 1 {
		t.Fatalf("function call must yield one chat chunk, got %d", len(frames))
	}
	call := gjson.GetBytes(frames[0], "choices.0.delta.tool_calls.0")
	if call.Get("id").String() != "call_01" || call.Get("function.name").String() != "shell" {
		t.Fatalf("tool call chunk = %s", frames[0])
	}

	frames = TranslateOpencodeResponsesStream("muse-spark-1.3-contributor-free", state, "response.function_call_arguments.delta", opencodeResponsesArgsDelta)
	if len(frames) != 1 {
		t.Fatalf("arguments delta must yield one chat chunk, got %d", len(frames))
	}
	if got := gjson.GetBytes(frames[0], "choices.0.delta.tool_calls.0.function.arguments").String(); got != `{"q":"ls -la"}` {
		t.Fatalf("arguments delta = %q", got)
	}

	frames = TranslateOpencodeResponsesStream("muse-spark-1.3-contributor-free", state, "response.completed", opencodeResponsesCompleted)
	if len(frames) != 2 {
		t.Fatalf("completed must yield usage plus finish chunks, got %d", len(frames))
	}
	if got := gjson.GetBytes(frames[0], "usage.total_tokens").Int(); got != 1121 {
		t.Fatalf("usage chunk total_tokens = %d", got)
	}
	if got := gjson.GetBytes(frames[1], "choices.0.finish_reason").String(); got != "tool_calls" {
		t.Fatalf("finish reason = %q, want tool_calls", got)
	}

	if frames := TranslateOpencodeResponsesStream("m", state, "response.created", `{"type":"response.created"}`); len(frames) != 0 {
		t.Fatal("lifecycle events must not emit chat frames")
	}
	if frames := TranslateOpencodeResponsesStream("m", nil, "response.output_text.delta", opencodeResponsesTextDelta); len(frames) != 0 {
		t.Fatal("nil state must yield no frames")
	}
}

func TestAssembleOpencodeResponsesCompletion(t *testing.T) {
	sse := "event: response.output_text.delta\ndata: " + opencodeResponsesTextDelta + "\n\n" +
		"event: response.output_item.added\ndata: " + opencodeResponsesCallAdded + "\n\n" +
		"event: response.function_call_arguments.delta\ndata: " + opencodeResponsesArgsDelta + "\n\n" +
		"event: response.completed\ndata: " + opencodeResponsesCompleted + "\n\n" +
		"data: [DONE]\n"
	out := AssembleOpencodeResponsesCompletion([]byte(sse), "muse-spark-1.3-contributor-free")
	if gjson.GetBytes(out, "object").String() != "chat.completion" {
		t.Fatalf("assembled body must be a completion, got %s", out)
	}
	if got := gjson.GetBytes(out, "choices.0.message.content").String(); got != "I'll list t" {
		t.Fatalf("assembled content = %q", got)
	}
	call := gjson.GetBytes(out, "choices.0.message.tool_calls.0")
	if call.Get("id").String() != "call_01" || call.Get("function.arguments").String() != `{"q":"ls -la"}` {
		t.Fatalf("assembled tool call = %s", gjson.GetBytes(out, "choices.0.message.tool_calls").Raw)
	}
	if got := gjson.GetBytes(out, "choices.0.finish_reason").String(); got != "tool_calls" {
		t.Fatalf("assembled finish reason = %q", got)
	}
	if got := gjson.GetBytes(out, "usage.total_tokens").Int(); got != 1121 {
		t.Fatalf("assembled usage total_tokens = %d", got)
	}

	plain := []byte(`{"error":"nope"}`)
	if out := AssembleOpencodeResponsesCompletion(plain, "m"); string(out) != string(plain) {
		t.Fatal("non-SSE bodies must be returned untouched")
	}
}

func TestNewOpencodeResponsesChatStream(t *testing.T) {
	upstream := "event: response.output_text.delta\ndata: " + opencodeResponsesTextDelta + "\n\n" +
		"event: response.completed\ndata: " + opencodeResponsesCompleted + "\n\n" +
		"data: [DONE]\n"
	stream := NewOpencodeResponsesChatStream(context.Background(), io.NopCloser(strings.NewReader(upstream)), "muse-spark-1.3-contributor-free")
	raw, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, `"content":"I'll list t"`) {
		t.Fatalf("translated stream must carry text, got %s", text)
	}
	if !strings.Contains(text, `"total_tokens":1121`) {
		t.Fatalf("translated stream must carry usage, got %s", text)
	}
	if !strings.Contains(text, "data: [DONE]") {
		t.Fatalf("translated stream must forward [DONE], got %s", text)
	}
}
