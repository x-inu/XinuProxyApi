package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	proxyconfig "github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestPublicModelsEndpoint_DisabledByDefault(t *testing.T) {
	cfg := &proxyconfig.Config{
		SDKConfig: proxyconfig.SDKConfig{
			APIKeys: []string{"secret-key-123"},
		},
		PublicModelsEndpoint: false,
	}
	server := newTestServerWithConfig(t, cfg)

	// GET /v1/models without key should be rejected with 401
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for /v1/models when public models disabled, got %d", rr.Code)
	}

	// GET /v1beta/models without key should be rejected with 401
	reqBeta := httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	rrBeta := httptest.NewRecorder()
	server.engine.ServeHTTP(rrBeta, reqBeta)
	if rrBeta.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for /v1beta/models when public models disabled, got %d", rrBeta.Code)
	}
}

func TestPublicModelsEndpoint_Enabled(t *testing.T) {
	cfg := &proxyconfig.Config{
		SDKConfig: proxyconfig.SDKConfig{
			APIKeys: []string{"secret-key-123"},
		},
		PublicModelsEndpoint: true,
	}
	server := newTestServerWithConfig(t, cfg)

	// GET /v1/models without key should return 200
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /v1/models when public models enabled, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// GET /v1beta/models without key should return 200
	reqBeta := httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	rrBeta := httptest.NewRecorder()
	server.engine.ServeHTTP(rrBeta, reqBeta)
	if rrBeta.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /v1beta/models when public models enabled, got %d (body: %s)", rrBeta.Code, rrBeta.Body.String())
	}

	// POST /v1/chat/completions without key must STILL return 401
	reqChat := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rrChat := httptest.NewRecorder()
	server.engine.ServeHTTP(rrChat, reqChat)
	if rrChat.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for /v1/chat/completions even when public models enabled, got %d", rrChat.Code)
	}

	// GET /v1/models with explicitly invalid key should be rejected with 401
	reqBadKey := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	reqBadKey.Header.Set("Authorization", "Bearer invalid-key")
	rrBadKey := httptest.NewRecorder()
	server.engine.ServeHTTP(rrBadKey, reqBadKey)
	if rrBadKey.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for /v1/models with invalid key, got %d", rrBadKey.Code)
	}
}
