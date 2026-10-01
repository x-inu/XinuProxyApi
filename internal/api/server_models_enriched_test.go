package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	proxyconfig "github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
)

// extractJSONKeyOrder parses a JSON object slice and returns the exact ordered list of keys for each object.
func extractModelKeyOrder(body []byte, targetModelID string) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var keys []string
	inData := false
	inTargetModel := false
	depth := 0

	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch v := t.(type) {
		case json.Delim:
			if v == '{' {
				depth++
			} else if v == '}' {
				if inTargetModel && depth == 2 {
					return keys, nil
				}
				depth--
			} else if v == '[' {
				// array start
			} else if v == ']' {
				// array end
			}
		case string:
			if v == "data" && depth == 1 {
				inData = true
			} else if inData && depth == 2 {
				// We are reading a key in a model object
				key := v
				// Read value to see if this is our target model
				var val any
				if err := dec.Decode(&val); err != nil {
					return nil, err
				}
				if key == "id" && val == targetModelID {
					inTargetModel = true
					keys = append(keys, key)
				} else if inTargetModel {
					keys = append(keys, key)
				}
			}
		}
	}
	return keys, nil
}

func extractSubObjectKeyOrder(body []byte, targetModelID, subObjectName string) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var keys []string
	inTargetModel := false
	inSubObject := false
	depth := 0

	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch v := t.(type) {
		case json.Delim:
			if v == '{' {
				depth++
			} else if v == '}' {
				if inSubObject {
					return keys, nil
				}
				depth--
			}
		case string:
			if depth == 2 && v == "id" {
				var idVal string
				if err := dec.Decode(&idVal); err != nil {
					return nil, err
				}
				if idVal == targetModelID {
					inTargetModel = true
				}
			} else if inTargetModel && depth == 2 && v == subObjectName {
				// next token must be '{'
				inSubObject = true
			} else if inSubObject && depth == 3 {
				key := v
				var val any
				if err := dec.Decode(&val); err != nil {
					return nil, err
				}
				keys = append(keys, key)
			}
		}
	}
	return keys, nil
}

func TestOpenAIModels_ReturnsEnrichedMetadataWithExactFieldOrder(t *testing.T) {
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
	if targetModel["thinking"] != true {
		t.Fatalf("expected thinking true, got %v", targetModel["thinking"])
	}
	if targetModel["supportsThinking"] != true {
		t.Fatalf("expected supportsThinking true, got %v", targetModel["supportsThinking"])
	}

	// Verify EXACT field order in serialized JSON
	expectedFieldOrder := []string{
		"id",
		"object",
		"created",
		"owned_by",
		"permission",
		"root",
		"parent",
		"capabilities",
		"input_modalities",
		"output_modalities",
		"thinking",
		"supportsThinking",
		"context_length",
		"max_output_tokens",
		"max_input_tokens",
		"name",
		"pricing",
	}

	keys, err := extractModelKeyOrder(rr.Body.Bytes(), "antigravity/gemini-3.7-flash-medium")
	if err != nil {
		t.Fatalf("failed to extract key order: %v", err)
	}

	if len(keys) != len(expectedFieldOrder) {
		t.Fatalf("expected %d keys in exact order, got %d keys: %v", len(expectedFieldOrder), len(keys), keys)
	}
	for i, want := range expectedFieldOrder {
		if keys[i] != want {
			t.Errorf("field order mismatch at index %d: got %q, want %q (full order: %v)", i, keys[i], want, keys)
		}
	}

	// Verify EXACT key order in capabilities
	expectedCapOrder := []string{"thinking", "supportsThinking", "vision", "tool_calling", "reasoning"}
	capKeys, errCap := extractSubObjectKeyOrder(rr.Body.Bytes(), "antigravity/gemini-3.7-flash-medium", "capabilities")
	if errCap != nil {
		t.Fatalf("failed to extract capabilities key order: %v", errCap)
	}
	for i, want := range expectedCapOrder {
		if i >= len(capKeys) || capKeys[i] != want {
			t.Errorf("capabilities order mismatch at index %d: got %v, want %v", i, capKeys, expectedCapOrder)
			break
		}
	}

	// Verify EXACT key order in pricing
	expectedPricingOrder := []string{"input", "output", "cached", "reasoning", "cache_creation"}
	pricingKeys, errPrice := extractSubObjectKeyOrder(rr.Body.Bytes(), "antigravity/gemini-3.7-flash-medium", "pricing")
	if errPrice != nil {
		t.Fatalf("failed to extract pricing key order: %v", errPrice)
	}
	for i, want := range expectedPricingOrder {
		if i >= len(pricingKeys) || pricingKeys[i] != want {
			t.Errorf("pricing order mismatch at index %d: got %v, want %v", i, pricingKeys, expectedPricingOrder)
			break
		}
	}
}

func TestOpenAIModels_KimiK3ExactFormat(t *testing.T) {
	modelRegistry := registry.GetGlobalRegistry()
	clientID := "test-client-kimi-k3"
	modelRegistry.RegisterClient(clientID, "kimi", registry.GetKimiModels())
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

	var target map[string]any
	for _, m := range response.Data {
		if m["id"] == "kimi-k3" {
			target = m
			break
		}
	}
	if target == nil {
		t.Fatalf("kimi-k3 not found in response: %s", rr.Body.String())
	}

	if target["id"] != "kimi-k3" {
		t.Errorf("id = %v, want kimi-k3", target["id"])
	}
	if target["object"] != "model" {
		t.Errorf("object = %v, want model", target["object"])
	}
	if target["owned_by"] != "moonshot" {
		t.Errorf("owned_by = %v, want moonshot", target["owned_by"])
	}
	if target["root"] != "kimi-k3" {
		t.Errorf("root = %v, want kimi-k3", target["root"])
	}
	if target["parent"] != nil {
		t.Errorf("parent = %v, want nil", target["parent"])
	}
	if target["name"] != "Kimi K3" {
		t.Errorf("name = %v, want Kimi K3", target["name"])
	}
	if target["context_length"] != float64(1048576) {
		t.Errorf("context_length = %v, want 1048576", target["context_length"])
	}
	if target["max_output_tokens"] != float64(65536) {
		t.Errorf("max_output_tokens = %v, want 65536", target["max_output_tokens"])
	}
	if target["max_input_tokens"] != float64(1048576) {
		t.Errorf("max_input_tokens = %v, want 1048576", target["max_input_tokens"])
	}

	// Verify exact field order for kimi-k3
	expectedOrder := []string{
		"id",
		"object",
		"created",
		"owned_by",
		"permission",
		"root",
		"parent",
		"capabilities",
		"input_modalities",
		"output_modalities",
		"thinking",
		"supportsThinking",
		"context_length",
		"max_output_tokens",
		"max_input_tokens",
		"name",
		"pricing",
	}

	keys, err := extractModelKeyOrder(rr.Body.Bytes(), "kimi-k3")
	if err != nil {
		t.Fatalf("extract key order failed: %v", err)
	}
	if len(keys) != len(expectedOrder) {
		t.Fatalf("expected %d keys, got %d: %v", len(expectedOrder), len(keys), keys)
	}
	for i, want := range expectedOrder {
		if keys[i] != want {
			t.Errorf("kimi-k3 key order mismatch at index %d: got %q, want %q", i, keys[i], want)
		}
	}
}

func TestOpenAIModels_OmitsUnknownFieldsAndPreservesOrder(t *testing.T) {
	modelRegistry := registry.GetGlobalRegistry()
	clientID := "test-client-unknown-model"
	modelRegistry.RegisterClient(clientID, "custom-unknown", []*registry.ModelInfo{
		{
			ID:      "custom-unknown/bare-model",
			OwnedBy: "custom-corp",
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

	keys, err := extractModelKeyOrder(rr.Body.Bytes(), "custom-unknown/bare-model")
	if err != nil {
		t.Fatalf("failed to extract key order: %v", err)
	}

	// Verify that present keys appear in the exact relative master order
	masterOrder := []string{
		"id",
		"object",
		"created",
		"owned_by",
		"permission",
		"root",
		"parent",
		"capabilities",
		"input_modalities",
		"output_modalities",
		"thinking",
		"supportsThinking",
		"context_length",
		"max_output_tokens",
		"max_input_tokens",
		"name",
		"pricing",
	}

	posInMaster := func(key string) int {
		for i, m := range masterOrder {
			if m == key {
				return i
			}
		}
		return -1
	}

	lastPos := -1
	for _, k := range keys {
		pos := posInMaster(k)
		if pos == -1 {
			t.Fatalf("unexpected key %q found in response", k)
		}
		if pos <= lastPos {
			t.Errorf("key %q at master pos %d appeared after master pos %d (keys: %v)", k, pos, lastPos, keys)
		}
		lastPos = pos
	}
}
