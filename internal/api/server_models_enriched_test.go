package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	proxyconfig "github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
)

func TestOpenAIModels_ReturnsEnrichedMetadata(t *testing.T) {
	modelRegistry := registry.GetGlobalRegistry()
	clientID := "test-client-enriched-models"
	modelRegistry.RegisterClient(clientID, "antigravity", []*registry.ModelInfo{
		{
			ID:                  "antigravity/gemini-3.7-flash-medium",
			DisplayName:         "Gemini 3.7 Flash (Medium)",
			OwnedBy:             "antigravity",
			ContextLength:       1048576,
			MaxCompletionTokens: 65536,
			ExplicitThinking:    true,
			SupportedInputModalities: []string{
				"text",
				"image",
			},
		},
	})
	t.Cleanup(func() {
		modelRegistry.UnregisterClient(clientID)
	})

	cfg := &proxyconfig.Config{
		SDKConfig: proxyconfig.SDKConfig{
			APIKeys: []string{"test-key"},
		},
		PublicModelsEndpoint: true,
	}
	server := newTestServerWithConfig(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body = %s", rr.Code, rr.Body.String())
	}

	var response struct {
		Object string           `json:"object"`
		Data   []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Object != "list" {
		t.Fatalf("expected object list, got %q", response.Object)
	}

	var targetModel map[string]any
	for _, m := range response.Data {
		if m["id"] == "antigravity/gemini-3.7-flash-medium" {
			targetModel = m
			break
		}
	}

	if targetModel == nil {
		t.Fatalf("target model not found in response: %s", rr.Body.String())
	}

	// Verify that created field is NOT present
	if _, exists := targetModel["created"]; exists {
		t.Fatalf("expected created field to be omitted, but it exists: %v", targetModel["created"])
	}

	// Verify required enriched fields
	if targetModel["root"] != "gemini-3.7-flash-medium" {
		t.Fatalf("expected root gemini-3.7-flash-medium, got %v", targetModel["root"])
	}
	if targetModel["name"] != "Gemini 3.7 Flash (Medium)" {
		t.Fatalf("expected name Gemini 3.7 Flash (Medium), got %v", targetModel["name"])
	}
	if targetModel["owned_by"] != "antigravity" {
		t.Fatalf("expected owned_by antigravity, got %v", targetModel["owned_by"])
	}
	if targetModel["context_length"] != float64(1048576) {
		t.Fatalf("expected context_length 1048576, got %v", targetModel["context_length"])
	}
	if targetModel["max_output_tokens"] != float64(65536) {
		t.Fatalf("expected max_output_tokens 65536, got %v", targetModel["max_output_tokens"])
	}
	if targetModel["max_input_tokens"] != float64(1048576) {
		t.Fatalf("expected max_input_tokens 1048576, got %v", targetModel["max_input_tokens"])
	}

	// Capabilities
	caps, ok := targetModel["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("expected capabilities object, got %T (%v)", targetModel["capabilities"], targetModel["capabilities"])
	}
	if caps["thinking"] != true {
		t.Fatalf("expected capabilities.thinking true, got %v", caps["thinking"])
	}
	if caps["vision"] != true {
		t.Fatalf("expected capabilities.vision true, got %v", caps["vision"])
	}
	if caps["tool_calling"] != true {
		t.Fatalf("expected capabilities.tool_calling true, got %v", caps["tool_calling"])
	}

	// Pricing
	pricing, ok := targetModel["pricing"].(map[string]any)
	if !ok {
		t.Fatalf("expected pricing object, got %T (%v)", targetModel["pricing"], targetModel["pricing"])
	}
	if _, exists := pricing["input"]; !exists {
		t.Fatalf("expected pricing.input to exist")
	}
	if _, exists := pricing["output"]; !exists {
		t.Fatalf("expected pricing.output to exist")
	}
}

func TestOpenAIModels_OmitsUnknownFieldsLikeOmniRoute(t *testing.T) {
	modelRegistry := registry.GetGlobalRegistry()
	clientID := "test-client-omit-unknown"
	modelRegistry.RegisterClient(clientID, "openai-compatible-unknown", []*registry.ModelInfo{
		{
			ID:      "unknown-provider/mystery-model",
			OwnedBy: "unknown",
		},
	})
	t.Cleanup(func() {
		modelRegistry.UnregisterClient(clientID)
	})

	cfg := &proxyconfig.Config{
		SDKConfig: proxyconfig.SDKConfig{
			APIKeys: []string{"test-key"},
		},
		PublicModelsEndpoint: true,
	}
	server := newTestServerWithConfig(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body = %s", rr.Code, rr.Body.String())
	}

	var response struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var target map[string]any
	for _, m := range response.Data {
		if m["id"] == "unknown-provider/mystery-model" {
			target = m
			break
		}
	}
	if target == nil {
		t.Fatalf("target model not found")
	}

	// Fields that should be OMITTED for unknown models (like OmniRoute)
	for _, field := range []string{"name", "pricing", "thinking", "supportsThinking",
		"max_input_tokens", "max_output_tokens", "context_length", "input_modalities"} {
		if _, exists := target[field]; exists {
			t.Errorf("expected field %q to be omitted for unknown model, but it exists: %v", field, target[field])
		}
	}

	// Fields that should ALWAYS be present
	for _, field := range []string{"id", "object", "owned_by", "permission", "root", "capabilities"} {
		if _, exists := target[field]; !exists {
			t.Errorf("expected field %q to always be present, but it is missing", field)
		}
	}

	// Capabilities should only have true values
	caps := target["capabilities"].(map[string]any)
	for k, v := range caps {
		if v != true {
			t.Errorf("capabilities.%s = %v, want only true values in capabilities", k, v)
		}
	}
}
