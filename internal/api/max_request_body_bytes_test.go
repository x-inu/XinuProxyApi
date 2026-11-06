package api

import (
	"bytes"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/clienterror"
	proxyconfig "github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/api/handlers"
)

type countingBodyReader struct {
	reader io.Reader
	read   atomic.Int64
}

func (r *countingBodyReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.read.Add(int64(n))
	}
	return n, err
}

func parseTestServerConfigYAML(t *testing.T, raw string) *proxyconfig.Config {
	t.Helper()
	cfg, errParse := proxyconfig.ParseConfigBytes([]byte(raw))
	if errParse != nil {
		t.Fatalf("ParseConfigBytes() error = %v", errParse)
	}
	return cfg
}

func TestMaxRequestBodyBytesConfig(t *testing.T) {
	// Flat config field: no nesting, default disabled.
	cfg := parseTestServerConfigYAML(t, "max-request-body-bytes: 64\n")
	if cfg.MaxRequestBodyBytes != 64 {
		t.Fatalf("cfg.MaxRequestBodyBytes = %d, want 64", cfg.MaxRequestBodyBytes)
	}

	defaultCfg := parseTestServerConfigYAML(t, "api-keys:\n  - test-key\n")
	if defaultCfg.MaxRequestBodyBytes != 0 {
		t.Fatalf("defaultCfg.MaxRequestBodyBytes = %d, want 0 (disabled)", defaultCfg.MaxRequestBodyBytes)
	}
}

func TestServerMaxRequestBodyBytesContentLength(t *testing.T) {
	cfg := parseTestServerConfigYAML(t, `max-request-body-bytes: 64
api-keys:
  - test-key
`)
	server := newTestServerWithConfig(t, cfg)

	t.Run("content-length exceeding limit returns 413 without reading body", func(t *testing.T) {
		counter := &countingBodyReader{reader: strings.NewReader(strings.Repeat("a", 128))}
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", counter)
		req.ContentLength = 128
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		server.engine.ServeHTTP(rr, req)

		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusRequestEntityTooLarge, rr.Body.String())
		}
		if got := counter.read.Load(); got != 0 {
			t.Fatalf("bytes read = %d, want 0", got)
		}
	})

	t.Run("exact boundary body is accepted", func(t *testing.T) {
		exactPayload := `{"model":"gpt-5","input":"ok"}`
		cfgExact := parseTestServerConfigYAML(t, "max-request-body-bytes: 30\napi-keys:\n  - test-key\n")
		srvExact := newTestServerWithConfig(t, cfgExact)
		if len(exactPayload) != 30 {
			t.Fatalf("len(exactPayload) = %d, want 30", len(exactPayload))
		}
		reqExact := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(exactPayload))
		reqExact.Header.Set("Authorization", "Bearer test-key")
		reqExact.Header.Set("Content-Type", "application/json")
		rrExact := httptest.NewRecorder()
		srvExact.engine.ServeHTTP(rrExact, reqExact)
		if rrExact.Code != http.StatusBadRequest || !strings.Contains(rrExact.Body.String(), "unknown provider for model gpt-5") {
			t.Fatalf("exact boundary status = %d body = %s, want 400 model_not_found", rrExact.Code, rrExact.Body.String())
		}
	})

	t.Run("negative limit disables request body cap", func(t *testing.T) {
		cfgNegative := parseTestServerConfigYAML(t, "max-request-body-bytes: -1\napi-keys:\n  - test-key\n")
		srvNegative := newTestServerWithConfig(t, cfgNegative)
		payload := `{"model":"gpt-5","input":"` + strings.Repeat("a", 256) + `"}`
		reqNeg := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(payload))
		reqNeg.Header.Set("Authorization", "Bearer test-key")
		reqNeg.Header.Set("Content-Type", "application/json")
		rrNeg := httptest.NewRecorder()
		srvNegative.engine.ServeHTTP(rrNeg, reqNeg)
		if rrNeg.Code != http.StatusBadRequest || !strings.Contains(rrNeg.Body.String(), "unknown provider for model gpt-5") {
			t.Fatalf("negative limit status = %d body = %s, want 400 model_not_found", rrNeg.Code, rrNeg.Body.String())
		}
	})

	t.Run("default off allows oversized body", func(t *testing.T) {
		cfgDefault := parseTestServerConfigYAML(t, "api-keys:\n  - test-key\n")
		srv := newTestServerWithConfig(t, cfgDefault)
		payload := `{"model":"gpt-5","messages":[{"role":"user","content":"` + strings.Repeat("d", 128) + `"}]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer test-key")
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.engine.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "unknown provider for model gpt-5") {
			t.Fatalf("default-off status = %d body = %s, want 400 model_not_found", rr.Code, rr.Body.String())
		}
	})
}

func TestMaxRequestBodyBytesMiddlewareChunkedBoundsBody(t *testing.T) {
	cfg := parseTestServerConfigYAML(t, `max-request-body-bytes: 64
api-keys:
  - test-key
`)
	srv := newTestServerWithConfig(t, cfg)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(srv.maxRequestBodyBytesMiddleware())
	router.POST("/test", func(c *gin.Context) {
		_, errRead := io.ReadAll(c.Request.Body)
		if errRead != nil {
			status := clienterror.HTTPStatusFromErrorOr(errRead, http.StatusBadRequest)
			c.JSON(status, gin.H{"error": errRead.Error()})
			return
		}
		c.Status(http.StatusOK)
	})

	counter := &countingBodyReader{reader: strings.NewReader(strings.Repeat("b", 1024))}
	req := httptest.NewRequest(http.MethodPost, "/test", io.NopCloser(counter))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusRequestEntityTooLarge, rr.Body.String())
	}
	if got := counter.read.Load(); got > 65 {
		t.Fatalf("bytes read before rejection = %d, want <= 65", got)
	}
}

func TestReadRequestBodyZstdLimit(t *testing.T) {
	compress := func(t *testing.T, raw []byte) []byte {
		t.Helper()
		var buf bytes.Buffer
		enc, errEnc := zstd.NewWriter(&buf)
		if errEnc != nil {
			t.Fatalf("zstd.NewWriter() error = %v", errEnc)
		}
		if _, errWrite := enc.Write(raw); errWrite != nil {
			t.Fatalf("Write() error = %v", errWrite)
		}
		if errClose := enc.Close(); errClose != nil {
			t.Fatalf("Close() error = %v", errClose)
		}
		return buf.Bytes()
	}

	newContext := func(compressed []byte, maxBytes int64) *gin.Context {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(compressed))
		c.Request.Header.Set("Content-Encoding", "zstd")
		c.Request.Header.Set("Content-Type", "application/json")
		if maxBytes > 0 {
			c.Set(handlers.MaxRequestBodyBytesContextKey, maxBytes)
		}
		return c
	}

	t.Run("decompressed body exceeding limit returns MaxBytesError mapped to 413", func(t *testing.T) {
		uncompressed := []byte(`{"model":"gpt-5","input":"` + strings.Repeat("z", 512) + `"}`)
		compressed := compress(t, uncompressed)
		if len(compressed) >= 128 {
			t.Fatalf("compressed size = %d, want < 128", len(compressed))
		}
		c := newContext(compressed, 128)
		_, errRead := handlers.ReadRequestBody(c)
		if errRead == nil {
			t.Fatal("ReadRequestBody() error = nil, want MaxBytesError")
		}
		var maxBytesErr *http.MaxBytesError
		if !errors.As(errRead, &maxBytesErr) {
			t.Fatalf("ReadRequestBody() error = %v, want *http.MaxBytesError", errRead)
		}
		if status := clienterror.HTTPStatusFromError(errRead); status != http.StatusRequestEntityTooLarge {
			t.Fatalf("HTTPStatusFromError() = %d, want %d", status, http.StatusRequestEntityTooLarge)
		}
	})

	t.Run("small zstd body succeeds", func(t *testing.T) {
		exactPayload := `{"model":"gpt-5","input":"ok"}`
		compressed := compress(t, []byte(exactPayload))
		c := newContext(compressed, 128)
		decoded, errRead := handlers.ReadRequestBody(c)
		if errRead != nil {
			t.Fatalf("ReadRequestBody() error = %v", errRead)
		}
		if string(decoded) != exactPayload {
			t.Fatalf("decoded = %q, want %q", decoded, exactPayload)
		}
	})

	t.Run("MaxInt64 limit does not overflow LimitReader", func(t *testing.T) {
		exactPayload := `{"model":"gpt-5","input":"ok"}`
		compressed := compress(t, []byte(exactPayload))
		c := newContext(compressed, math.MaxInt64)
		decoded, errRead := handlers.ReadRequestBody(c)
		if errRead != nil {
			t.Fatalf("ReadRequestBody() error = %v", errRead)
		}
		if string(decoded) != exactPayload {
			t.Fatalf("decoded = %q, want %q", decoded, exactPayload)
		}
	})
}
