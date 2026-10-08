package helps

import (
	"bufio"
	"context"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Narrow OpenCode zen bridge for responses-protocol free models (muse-spark).
// The upstream serves these on POST /v1/responses with the standard OpenAI
// Responses shape, while proxy clients speak chat/completions. This file maps
// chat requests to responses requests and folds responses SSE back into chat
// SSE (streaming clients) or a chat completion (non-streaming clients). The
// free-tier unlock recipe (versioned identity, minted session, read+shell
// markers, forced streaming) applies unchanged; only the wire shape differs.
const opencodeResponsesEndpoint = "/responses"

// OpencodeResponsesEndpoint returns the upstream responses path for
// responses-protocol models.
func OpencodeResponsesEndpoint() string {
	return opencodeResponsesEndpoint
}

// OpencodeResponsesProtocolModel reports whether an upstream free model is
// served on the responses protocol instead of chat/completions.
func OpencodeResponsesProtocolModel(model string) bool {
	trimmed := strings.TrimSpace(model)
	if strings.HasPrefix(trimmed, "muse-spark-") {
		return true
	}
	return strings.Contains(trimmed, "/muse-spark-")
}

// BuildOpencodeResponsesRequest converts a translated OpenAI chat payload
// into an OpenAI Responses request body. Streaming is always forced because
// the free tier only serves streams.
func BuildOpencodeResponsesRequest(chatPayload []byte) []byte {
	out := []byte(`{"stream":true}`)
	var err error
	model := strings.TrimSpace(gjson.GetBytes(chatPayload, "model").String())
	if model == "" {
		return chatPayload
	}
	if out, err = sjson.SetBytes(out, "model", model); err != nil {
		return chatPayload
	}
	if instructions := opencodeResponsesInstructions(chatPayload); instructions != "" {
		if out, err = sjson.SetBytes(out, "instructions", instructions); err != nil {
			return chatPayload
		}
	}
	if input := opencodeResponsesInput(chatPayload); len(input) > 0 {
		if out, err = sjson.SetRawBytes(out, "input", input); err != nil {
			return chatPayload
		}
	}
	if tools := opencodeResponsesTools(chatPayload); len(tools) > 0 {
		if out, err = sjson.SetRawBytes(out, "tools", tools); err != nil {
			return chatPayload
		}
	}
	if choice := opencodeResponsesToolChoice(chatPayload); len(choice) > 0 {
		if out, err = sjson.SetRawBytes(out, "tool_choice", choice); err != nil {
			return chatPayload
		}
	}
	if maxOut := opencodeResponsesMaxOutputTokens(chatPayload); maxOut.Exists() {
		if out, err = sjson.SetRawBytes(out, "max_output_tokens", []byte(maxOut.Raw)); err != nil {
			return chatPayload
		}
	}
	for _, field := range []string{"temperature", "top_p", "parallel_tool_calls"} {
		if value := gjson.GetBytes(chatPayload, field); value.Exists() {
			if out, err = sjson.SetRawBytes(out, field, []byte(value.Raw)); err != nil {
				return chatPayload
			}
		}
	}
	return out
}

// opencodeResponsesTextOf extracts plain text from a chat content value,
// which may be a string or an array of parts carrying text.
func opencodeResponsesTextOf(content gjson.Result) string {
	if content.Type == gjson.String {
		return content.String()
	}
	if !content.IsArray() {
		return ""
	}
	var sb strings.Builder
	for _, part := range content.Array() {
		kind := strings.TrimSpace(part.Get("type").String())
		if kind != "text" && kind != "input_text" && kind != "output_text" {
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
	}
	return sb.String()
}

func opencodeResponsesInstructions(chatPayload []byte) string {
	var sb strings.Builder
	for _, msg := range gjson.GetBytes(chatPayload, "messages").Array() {
		role := strings.TrimSpace(msg.Get("role").String())
		if role != "system" && role != "developer" {
			continue
		}
		if text := opencodeResponsesTextOf(msg.Get("content")); text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(text)
		}
	}
	return sb.String()
}

func opencodeResponsesInput(chatPayload []byte) []byte {
	var sb strings.Builder
	sb.WriteByte('[')
	first := true
	appendRaw := func(raw string) {
		if !first {
			sb.WriteByte(',')
		}
		first = false
		sb.WriteString(raw)
	}
	for _, msg := range gjson.GetBytes(chatPayload, "messages").Array() {
		role := strings.TrimSpace(msg.Get("role").String())
		switch role {
		case "system", "developer":
			continue
		case "tool":
			callID := strings.TrimSpace(msg.Get("tool_call_id").String())
			if callID == "" {
				callID = strings.TrimSpace(msg.Get("id").String())
			}
			item, err := sjson.SetBytes([]byte(`{"type":"function_call_output"}`), "call_id", callID)
			if err != nil {
				return nil
			}
			output := opencodeResponsesTextOf(msg.Get("content"))
			if output == "" {
				output = strings.TrimSpace(msg.String())
			}
			item, err = sjson.SetBytes(item, "output", output)
			if err != nil {
				return nil
			}
			appendRaw(string(item))
		default:
			for _, call := range msg.Get("tool_calls").Array() {
				fn := call.Get("function")
				name := strings.TrimSpace(fn.Get("name").String())
				if name == "" {
					name = strings.TrimSpace(call.Get("name").String())
				}
				if name == "" {
					continue
				}
				item := []byte(`{"type":"function_call"}`)
				var err error
				if id := strings.TrimSpace(call.Get("id").String()); id != "" {
					if item, err = sjson.SetBytes(item, "call_id", id); err != nil {
						return nil
					}
				}
				if item, err = sjson.SetBytes(item, "name", name); err != nil {
					return nil
				}
				args := strings.TrimSpace(fn.Get("arguments").String())
				if args == "" {
					args = "{}"
				}
				// The responses API requires arguments as a JSON-encoded
				// string, never a raw object: quote it regardless of how the
				// client shaped it (string or already-parsed object).
				if item, err = sjson.SetBytes(item, "arguments", args); err != nil {
					return nil
				}
				appendRaw(string(item))
			}
			if text := opencodeResponsesTextOf(msg.Get("content")); text != "" {
				item, err := sjson.SetBytes([]byte(`{"role":"`+role+`"}`), "content", text)
				if err != nil {
					return nil
				}
				appendRaw(string(item))
			}
		}
	}
	sb.WriteByte(']')
	if first {
		return nil
	}
	return []byte(sb.String())
}

func opencodeResponsesTools(chatPayload []byte) []byte {
	tools := gjson.GetBytes(chatPayload, "tools")
	if !tools.IsArray() || len(tools.Array()) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteByte('[')
	first := true
	for _, tool := range tools.Array() {
		fn := tool.Get("function")
		name := strings.TrimSpace(fn.Get("name").String())
		if name == "" {
			name = strings.TrimSpace(tool.Get("name").String())
		}
		if name == "" {
			continue
		}
		item := []byte(`{"type":"function"}`)
		var err error
		if item, err = sjson.SetBytes(item, "name", name); err != nil {
			return nil
		}
		if desc := tool.Get("description"); desc.Exists() {
			if item, err = sjson.SetRawBytes(item, "description", []byte(desc.Raw)); err != nil {
				return nil
			}
		} else if desc := fn.Get("description"); desc.Exists() {
			if item, err = sjson.SetRawBytes(item, "description", []byte(desc.Raw)); err != nil {
				return nil
			}
		}
		params := fn.Get("parameters")
		if !params.Exists() {
			params = tool.Get("parameters")
		}
		if params.Exists() {
			if item, err = sjson.SetRawBytes(item, "parameters", []byte(params.Raw)); err != nil {
				return nil
			}
		}
		if strict := tool.Get("strict"); strict.Exists() {
			if item, err = sjson.SetRawBytes(item, "strict", []byte(strict.Raw)); err != nil {
				return nil
			}
		}
		if !first {
			sb.WriteByte(',')
		}
		first = false
		sb.WriteString(string(item))
	}
	sb.WriteByte(']')
	if first {
		return nil
	}
	return []byte(sb.String())
}

func opencodeResponsesToolChoice(chatPayload []byte) []byte {
	choice := gjson.GetBytes(chatPayload, "tool_choice")
	if !choice.Exists() {
		return nil
	}
	if choice.Type == gjson.String {
		switch strings.TrimSpace(choice.String()) {
		case "none", "auto", "required":
			return []byte(`"` + strings.TrimSpace(choice.String()) + `"`)
		}
		return nil
	}
	name := strings.TrimSpace(choice.Get("function.name").String())
	if name == "" {
		name = strings.TrimSpace(choice.Get("name").String())
	}
	if name == "" {
		return nil
	}
	item, err := sjson.SetBytes([]byte(`{"type":"function"}`), "name", name)
	if err != nil {
		return nil
	}
	return item
}

func opencodeResponsesMaxOutputTokens(chatPayload []byte) gjson.Result {
	if value := gjson.GetBytes(chatPayload, "max_completion_tokens"); value.Exists() {
		return value
	}
	return gjson.GetBytes(chatPayload, "max_tokens")
}

// EnsureOpencodeResponsesMarkerTools appends read+shell marker tools in
// Responses shape when either name is missing.
func EnsureOpencodeResponsesMarkerTools(responsesPayload []byte) []byte {
	tools := gjson.GetBytes(responsesPayload, "tools")
	if tools.Exists() && !tools.IsArray() {
		return responsesPayload
	}
	var hasRead, hasShell bool
	for _, tool := range tools.Array() {
		switch strings.TrimSpace(tool.Get("name").String()) {
		case "read":
			hasRead = true
		case "shell":
			hasShell = true
		}
	}
	if hasRead && hasShell {
		return responsesPayload
	}
	out := responsesPayload
	add := func(raw string) []byte {
		updated, err := sjson.SetRawBytes(out, "tools.-1", []byte(raw))
		if err != nil {
			return out
		}
		return updated
	}
	if !hasRead {
		out = add(`{"type":"function","name":"read","description":"read a file","parameters":{"type":"object","properties":{"p":{"type":"string"}}}}`)
	}
	if !hasShell {
		out = add(`{"type":"function","name":"shell","description":"run a command","parameters":{"type":"object","properties":{"q":{"type":"string"}}}}`)
	}
	return out
}

// opencodeResponsesChatState tracks folding state while translating a
// responses SSE stream into chat SSE frames.
type opencodeResponsesChatState struct {
	model       string
	created     int64
	sentRole    bool
	toolIndexes map[string]int
	itemIndexes map[string]int
	nextIndex   int
	sawToolCall bool
}

// TranslateOpencodeResponsesStream converts one upstream responses SSE frame
// (event name plus data payload) into zero or more chat SSE frame bodies.
// The caller frames each returned body as "data: <body>" and appends [DONE]
// when the upstream stream ends.
func TranslateOpencodeResponsesStream(model string, state *opencodeResponsesChatState, event, data string) [][]byte {
	if state == nil {
		return nil
	}
	if state.model == "" {
		state.model = model
	}
	if state.created == 0 {
		state.created = time.Now().Unix()
	}
	if state.toolIndexes == nil {
		state.toolIndexes = map[string]int{}
	}
	if state.itemIndexes == nil {
		state.itemIndexes = map[string]int{}
	}
	payload := gjson.Parse(data)
	switch event {
	case "response.output_text.delta":
		delta := payload.Get("delta").String()
		if delta == "" {
			return nil
		}
		return [][]byte{opencodeResponsesTextChunk(state, delta)}
	case "response.output_item.added":
		item := payload.Get("item")
		if strings.TrimSpace(item.Get("type").String()) != "function_call" {
			return nil
		}
		callID := strings.TrimSpace(item.Get("call_id").String())
		if callID == "" {
			callID = strings.TrimSpace(item.Get("id").String())
		}
		name := strings.TrimSpace(item.Get("name").String())
		index, ok := state.toolIndexes[callID]
		if !ok {
			index = state.nextIndex
			state.nextIndex++
			state.toolIndexes[callID] = index
		}
		if itemID := strings.TrimSpace(item.Get("id").String()); itemID != "" {
			state.itemIndexes[itemID] = index
		}
		state.sawToolCall = true
		return [][]byte{opencodeResponsesToolChunk(state, index, callID, name, "")}
	case "response.function_call_arguments.delta":
		itemID := strings.TrimSpace(payload.Get("item_id").String())
		delta := payload.Get("delta").String()
		if delta == "" {
			return nil
		}
		index, ok := state.itemIndexes[itemID]
		if !ok {
			index, ok = state.toolIndexes[itemID]
		}
		if !ok {
			index = state.nextIndex
			state.nextIndex++
			state.toolIndexes[itemID] = index
			state.itemIndexes[itemID] = index
		}
		state.sawToolCall = true
		return [][]byte{opencodeResponsesToolArgsChunk(state, index, delta)}
	case "response.completed":
		response := payload.Get("response")
		finish := "stop"
		if state.sawToolCall || opencodeResponsesOutputHasToolCall(response) {
			finish = "tool_calls"
		}
		var frames [][]byte
		if usage := response.Get("usage"); usage.Exists() && usage.IsObject() {
			if chunk, err := sjson.SetRawBytes(opencodeResponsesBaseChunk(state), "usage", []byte(usage.Raw)); err == nil {
				empty, err := sjson.SetRawBytes(chunk, "choices", []byte(`[]`))
				if err == nil {
					frames = append(frames, empty)
				}
			}
		}
		final, err := sjson.SetBytes(opencodeResponsesBaseChunk(state), "choices.0.finish_reason", finish)
		if err == nil {
			delta, err := sjson.SetRawBytes(final, "choices.0.delta", []byte(`{}`))
			if err == nil {
				frames = append(frames, delta)
			}
		}
		return frames
	case "response.failed", "response.incomplete":
		errText := strings.TrimSpace(payload.Get("response.error.message").String())
		if errText == "" {
			errText = strings.TrimSpace(payload.Get("error.message").String())
		}
		if errText == "" {
			errText = "upstream responses stream " + event
		}
		if chunk, err := sjson.SetBytes([]byte(`{"error":{}}`), "error.message", errText); err == nil {
			return [][]byte{chunk}
		}
		return nil
	default:
		return nil
	}
}

func opencodeResponsesBaseChunk(state *opencodeResponsesChatState) []byte {
	id := "chatcmpl-opencode-" + uuid.NewString()[:8]
	chunk, _ := sjson.SetBytes([]byte(`{"object":"chat.completion.chunk"}`), "id", id)
	chunk, _ = sjson.SetBytes(chunk, "created", state.created)
	chunk, _ = sjson.SetBytes(chunk, "model", state.model)
	chunk, _ = sjson.SetRawBytes(chunk, "choices", []byte(`[{"index":0,"delta":{},"finish_reason":null}]`))
	return chunk
}

func opencodeResponsesTextChunk(state *opencodeResponsesChatState, delta string) []byte {
	chunk := opencodeResponsesBaseChunk(state)
	if !state.sentRole {
		chunk, _ = sjson.SetBytes(chunk, "choices.0.delta.role", "assistant")
		state.sentRole = true
	}
	chunk, _ = sjson.SetBytes(chunk, "choices.0.delta.content", delta)
	return chunk
}

func opencodeResponsesToolChunk(state *opencodeResponsesChatState, index int, callID, name, args string) []byte {
	chunk := opencodeResponsesBaseChunk(state)
	call := `{"index":` + itoa(index) + `,"id":` + quoteRaw(callID) + `,"type":"function","function":{"name":` + quoteRaw(name) + `,"arguments":` + quoteRaw(args) + `}}`
	chunk, _ = sjson.SetRawBytes(chunk, "choices.0.delta.tool_calls", []byte(`[`+call+`]`))
	return chunk
}

func opencodeResponsesToolArgsChunk(state *opencodeResponsesChatState, index int, delta string) []byte {
	chunk := opencodeResponsesBaseChunk(state)
	call := `{"index":` + itoa(index) + `,"function":{"arguments":` + quoteRaw(delta) + `}}`
	chunk, _ = sjson.SetRawBytes(chunk, "choices.0.delta.tool_calls", []byte(`[`+call+`]`))
	return chunk
}

func opencodeResponsesOutputHasToolCall(response gjson.Result) bool {
	for _, item := range response.Get("output").Array() {
		if strings.TrimSpace(item.Get("type").String()) == "function_call" {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func quoteRaw(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

// NewOpencodeResponsesChatStream wraps an upstream responses SSE body so
// readers observe chat SSE frames. The wrapper aborts promptly when ctx is
// done so abandoned streams cannot wedge the translating goroutine.
func NewOpencodeResponsesChatStream(ctx context.Context, body io.ReadCloser, model string) io.ReadCloser {
	if ctx == nil {
		ctx = context.Background()
	}
	pr, pw := io.Pipe()
	go func() {
		<-ctx.Done()
		pw.CloseWithError(ctx.Err())
	}()
	go func() {
		scanner := bufio.NewScanner(body)
		scanner.Buffer(nil, 52_428_800)
		state := &opencodeResponsesChatState{}
		var event string
		var dataLines []string
		// flush translates one buffered frame; false stops the loop.
		flush := func() bool {
			if len(dataLines) == 0 {
				event = ""
				return true
			}
			data := strings.Join(dataLines, "\n")
			dataLines = nil
			current := event
			event = ""
			for _, frame := range TranslateOpencodeResponsesStream(model, state, current, data) {
				select {
				case <-ctx.Done():
					return false
				default:
				}
				if _, err := pw.Write(append(append([]byte("data: "), frame...), '\n', '\n')); err != nil {
					return false
				}
			}
			return true
		}
		running := true
		for running && scanner.Scan() {
			select {
			case <-ctx.Done():
				running = false
			default:
			}
			if !running {
				break
			}
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				running = flush()
				continue
			}
			if strings.HasPrefix(line, "event:") {
				event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "[DONE]" {
					running = flush()
					if running {
						if _, err := pw.Write([]byte("data: [DONE]\n\n")); err != nil {
							running = false
						}
					}
					continue
				}
				dataLines = append(dataLines, data)
				continue
			}
		}
		flush()
		_ = body.Close()
		_ = pw.Close()
	}()
	return &opencodeResponsesReadCloser{Reader: pr, Closer: body}
}

type opencodeResponsesReadCloser struct {
	io.Reader
	io.Closer
}

// AssembleOpencodeResponsesCompletion folds a responses SSE body into a
// single non-streaming chat completion.
func AssembleOpencodeResponsesCompletion(body []byte, model string) []byte {
	var text strings.Builder
	type toolCall struct {
		id, name string
		args     strings.Builder
	}
	calls := map[string]*toolCall{}
	aliases := map[string]string{}
	var order []string
	var usageRaw string
	finish := "stop"
	lines := strings.Split(string(body), "\n")
	var event string
	var dataLines []string
	flush := func() {
		if len(dataLines) == 0 {
			event = ""
			return
		}
		data := strings.Join(dataLines, "\n")
		dataLines = nil
		current := event
		event = ""
		payload := gjson.Parse(data)
		switch current {
		case "response.output_text.delta":
			text.WriteString(payload.Get("delta").String())
		case "response.output_item.added":
			item := payload.Get("item")
			if strings.TrimSpace(item.Get("type").String()) == "function_call" {
				callID := strings.TrimSpace(item.Get("call_id").String())
				if callID == "" {
					callID = strings.TrimSpace(item.Get("id").String())
				}
				if _, ok := calls[callID]; !ok {
					calls[callID] = &toolCall{id: callID, name: strings.TrimSpace(item.Get("name").String())}
					order = append(order, callID)
				}
				if itemID := strings.TrimSpace(item.Get("id").String()); itemID != "" {
					aliases[itemID] = callID
				}
			}
		case "response.function_call_arguments.delta":
			itemID := strings.TrimSpace(payload.Get("item_id").String())
			if target, ok := aliases[itemID]; ok {
				itemID = target
			}
			call, ok := calls[itemID]
			if !ok {
				call = &toolCall{id: itemID}
				calls[itemID] = call
				order = append(order, itemID)
			}
			call.args.WriteString(payload.Get("delta").String())
		case "response.completed":
			response := payload.Get("response")
			if usage := response.Get("usage"); usage.Exists() && usage.IsObject() {
				usageRaw = usage.Raw
			}
			if opencodeResponsesOutputHasToolCall(response) {
				finish = "tool_calls"
			}
			for _, item := range response.Get("output").Array() {
				if strings.TrimSpace(item.Get("type").String()) != "function_call" {
					continue
				}
				callID := strings.TrimSpace(item.Get("call_id").String())
				itemID := strings.TrimSpace(item.Get("id").String())
				call, ok := calls[callID]
				if !ok {
					call, ok = calls[itemID]
					if ok {
						delete(calls, itemID)
						calls[callID] = call
						for i, id := range order {
							if id == itemID {
								order[i] = callID
							}
						}
					}
				}
				if !ok {
					call = &toolCall{id: callID}
					if call.id == "" {
						call.id = itemID
					}
					calls[call.id] = call
					order = append(order, call.id)
				}
				if call.name == "" {
					call.name = strings.TrimSpace(item.Get("name").String())
				}
				if call.args.Len() == 0 {
					call.args.WriteString(strings.TrimSpace(item.Get("arguments").String()))
				}
			}
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data := strings.TrimSpace(strings.TrimPrefix(line, "data:")); data != "[DONE]" {
				dataLines = append(dataLines, data)
			}
			continue
		}
	}
	flush()
	if text.Len() == 0 && len(order) == 0 && usageRaw == "" {
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
	message := []byte(`{"role":"assistant"}`)
	if message, err = sjson.SetBytes(message, "content", text.String()); err != nil {
		return body
	}
	if len(order) > 0 {
		var callsJSON strings.Builder
		callsJSON.WriteByte('[')
		for i, id := range order {
			call := calls[id]
			if i > 0 {
				callsJSON.WriteByte(',')
			}
			callsJSON.WriteString(`{"index":` + itoa(i) + `,"id":` + quoteRaw(call.id) + `,"type":"function","function":{"name":` + quoteRaw(call.name) + `,"arguments":` + quoteRaw(call.args.String()) + `}}`)
		}
		callsJSON.WriteByte(']')
		if message, err = sjson.SetRawBytes(message, "tool_calls", []byte(callsJSON.String())); err != nil {
			return body
		}
	}
	choice := []byte(`{"index":0}`)
	if choice, err = sjson.SetRawBytes(choice, "message", message); err != nil {
		return body
	}
	if choice, err = sjson.SetBytes(choice, "finish_reason", finish); err != nil {
		return body
	}
	if out, err = sjson.SetRawBytes(out, "choices", []byte(`[`+string(choice)+`]`)); err != nil {
		return body
	}
	if usageRaw != "" {
		if out, err = sjson.SetRawBytes(out, "usage", []byte(usageRaw)); err != nil {
			return body
		}
	}
	return out
}
