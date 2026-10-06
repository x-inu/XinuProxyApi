package executor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestShouldForceClaudeNonStream(t *testing.T) {
	cases := []struct {
		name     string
		cfg      *config.Config
		auth     *cliproxyauth.Auth
		endpoint string
		want     bool
	}{
		{
			name:     "Anthropic official URL",
			endpoint: "https://api.anthropic.com/v1/messages",
			want:     false,
		},
		{
			name:     "Kimi URL",
			endpoint: "https://api.kimi.com/coding/v1/messages",
			want:     false,
		},
		{
			name:     "Justwoker URL",
			endpoint: "https://api.justwoker.icu/v1/messages",
			want:     true,
		},
		{
			name:     "Justwoker subdomain URL",
			endpoint: "https://test.justwoker.com/v1/messages",
			want:     true,
		},
		{
			name: "JDW prefix on auth",
			auth: &cliproxyauth.Auth{
				Prefix: "jdw",
			},
			endpoint: "https://custom-proxy.example.com/v1/messages",
			want:     true,
		},
		{
			name: "JDW prefix in auth attributes",
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{"prefix": "jdw"},
			},
			endpoint: "https://custom-proxy.example.com/v1/messages",
			want:     true,
		},
		{
			name: "Force non-stream in auth attributes",
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{"force_non_stream": "true"},
			},
			endpoint: "https://custom-proxy.example.com/v1/messages",
			want:     true,
		},
		{
			name: "Disable streaming in auth attributes",
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{"disable_streaming": "true"},
			},
			endpoint: "https://custom-proxy.example.com/v1/messages",
			want:     true,
		},
		{
			name: "ForceNonStream in config key",
			cfg: &config.Config{
				ClaudeKey: []config.ClaudeKey{
					{
						APIKey:         "sk-test-key",
						BaseURL:        "https://custom-proxy.example.com",
						ForceNonStream: true,
					},
				},
			},
			auth: &cliproxyauth.Auth{
				Attributes: map[string]string{"api_key": "sk-test-key"},
			},
			endpoint: "https://custom-proxy.example.com/v1/messages",
			want:     true,
		},
		{
			name:     "Unrelated custom gateway without flag",
			endpoint: "https://my-gateway.example.com/v1/messages",
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldForceClaudeNonStream(tc.cfg, tc.auth, tc.endpoint)
			if got != tc.want {
				t.Fatalf("shouldForceClaudeNonStream() = %v, want %v", got, tc.want)
			}
		})
	}
}

type jdwRoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn jdwRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestClaudeExecutor_Execute_JustwokerNonStreamFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	var capturedUpstreamBody []byte
	upstreamResponseJSON := `{
		"id": "msg_jdw_123",
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
	}`

	ctx := context.WithValue(context.Background(), "gin", ginCtx)
	ctx = context.WithValue(ctx, "cliproxy.roundtripper", jdwRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		capturedUpstreamBody, _ = io.ReadAll(req.Body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(upstreamResponseJSON)),
		}, nil
	}))

	cfg := &config.Config{}
	exec := NewClaudeExecutor(cfg)

	auth := &cliproxyauth.Auth{
		ID:       "jdw-test-auth",
		Provider: "claude",
		Prefix:   "jdw",
		Attributes: map[string]string{
			"api_key":  "sk-jdw-test",
			"base_url": "https://api.justwoker.icu",
		},
	}

	clientRequest := []byte(`{
		"model": "jdw/claude-opus-4-8",
		"messages": [{"role": "user", "content": "Reply exactly: TEST OK"}],
		"max_tokens": 50,
		"stream": false
	}`)

	resp, err := exec.Execute(ctx, auth, cliproxyexecutor.Request{
		Model:   "jdw/claude-opus-4-8",
		Payload: clientRequest,
	}, cliproxyexecutor.Options{
		Stream:          false,
		SourceFormat:    sdktranslator.FormatOpenAI,
		OriginalRequest: clientRequest,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// 1. Verify upstream request received stream: false.
	streamField := gjson.GetBytes(capturedUpstreamBody, "stream")
	if streamField.Bool() != false {
		t.Fatalf("upstream request stream = %v, want false", streamField.Bool())
	}

	// 2. Verify downstream OpenAI response contains the expected content.
	parsed := gjson.ParseBytes(resp.Payload)
	content := parsed.Get("choices.0.message.content").String()
	if content != "TEST OK" {
		t.Fatalf("choices.0.message.content = %q, want %q; full response: %s", content, "TEST OK", string(resp.Payload))
	}
	finishReason := parsed.Get("choices.0.finish_reason").String()
	if finishReason != "stop" {
		t.Fatalf("choices.0.finish_reason = %q, want %q", finishReason, "stop")
	}
}

func TestClaudeExecutor_ExecuteStream_JustwokerNonStreamFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	var capturedUpstreamBody []byte
	upstreamResponseJSON := `{
		"id": "msg_jdw_stream_456",
		"type": "message",
		"role": "assistant",
		"model": "claude-opus-4-8",
		"content": [{"type": "text", "text": "STREAM TEST OK"}],
		"stop_reason": "end_turn",
		"stop_sequence": null,
		"usage": {
			"input_tokens": 10375,
			"output_tokens": 7,
			"cache_creation_input_tokens": 10373
		}
	}`

	ctx := context.WithValue(context.Background(), "gin", ginCtx)
	ctx = context.WithValue(ctx, "cliproxy.roundtripper", jdwRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		capturedUpstreamBody, _ = io.ReadAll(req.Body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(upstreamResponseJSON)),
		}, nil
	}))

	cfg := &config.Config{}
	exec := NewClaudeExecutor(cfg)

	auth := &cliproxyauth.Auth{
		ID:       "jdw-test-auth",
		Provider: "claude",
		Prefix:   "jdw",
		Attributes: map[string]string{
			"api_key":  "sk-jdw-test",
			"base_url": "https://api.justwoker.icu",
		},
	}

	clientRequest := []byte(`{
		"model": "jdw/claude-opus-4-8",
		"messages": [{"role": "user", "content": "Reply exactly: TEST OK"}],
		"max_tokens": 50,
		"stream": true
	}`)

	streamResult, err := exec.ExecuteStream(ctx, auth, cliproxyexecutor.Request{
		Model:   "jdw/claude-opus-4-8",
		Payload: clientRequest,
	}, cliproxyexecutor.Options{
		Stream:          true,
		SourceFormat:    sdktranslator.FormatOpenAI,
		OriginalRequest: clientRequest,
	})
	if err != nil {
		t.Fatalf("ExecuteStream() returned error: %v", err)
	}

	// 1. Verify upstream request received stream: false.
	streamField := gjson.GetBytes(capturedUpstreamBody, "stream")
	if streamField.Bool() != false {
		t.Fatalf("upstream request stream = %v, want false", streamField.Bool())
	}

	// 2. Collect all streamed chunks and verify content delta.
	var receivedContent strings.Builder
	hasFinishReason := false
	for chunk := range streamResult.Chunks {
		if chunk.Err != nil {
			t.Fatalf("stream chunk error: %v", chunk.Err)
		}
		parsed := gjson.ParseBytes(chunk.Payload)
		if text := parsed.Get("choices.0.delta.content").String(); text != "" {
			receivedContent.WriteString(text)
		}
		if fr := parsed.Get("choices.0.finish_reason").String(); fr == "stop" {
			hasFinishReason = true
		}
	}

	if receivedContent.String() != "STREAM TEST OK" {
		t.Fatalf("streamed content = %q, want %q", receivedContent.String(), "STREAM TEST OK")
	}
	if !hasFinishReason {
		t.Fatal("expected finish_reason 'stop' in stream chunks")
	}
}
