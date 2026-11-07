package management

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/x-inu/XinuProxyApi/v7/internal/config"
	"github.com/x-inu/XinuProxyApi/v7/internal/logging"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestConsoleLogHandlers(t *testing.T) {
	logging.RegisterConsoleLogHook()
	logging.ClearConsoleBuffer()

	h := NewHandlerWithoutConfigFilePath(&config.Config{}, nil)

	// 1. Initially empty
	{
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/console-logs", nil)
		h.GetConsoleLogs(c)

		if rec.Code != http.StatusOK {
			t.Fatalf("GetConsoleLogs status = %d, want 200", rec.Code)
		}
		var resp struct {
			Success bool                   `json:"success"`
			Logs    []logging.ConsoleEntry `json:"logs"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Success {
			t.Fatal("expected success true")
		}
		if len(resp.Logs) != 0 {
			t.Fatalf("expected 0 logs, got %d", len(resp.Logs))
		}
	}

	// 2. Emit a log and verify it is captured
	log.WithField("request_id", "test-req-1").Info("Hello from test console log")

	{
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/console-logs", nil)
		h.GetConsoleLogs(c)

		if rec.Code != http.StatusOK {
			t.Fatalf("GetConsoleLogs status = %d, want 200", rec.Code)
		}
		var resp struct {
			Success bool                   `json:"success"`
			Logs    []logging.ConsoleEntry `json:"logs"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Logs) == 0 {
			t.Fatal("expected at least 1 log entry")
		}
		latest := resp.Logs[len(resp.Logs)-1]
		if latest.RequestID != "test-req-1" {
			t.Fatalf("expected request_id test-req-1, got %q", latest.RequestID)
		}
	}

	// 3. Clear logs
	{
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodDelete, "/console-logs", nil)
		h.DeleteConsoleLogs(c)

		if rec.Code != http.StatusOK {
			t.Fatalf("DeleteConsoleLogs status = %d, want 200", rec.Code)
		}
		if len(logging.GetConsoleBuffer()) != 0 {
			t.Fatal("expected buffer to be empty after delete")
		}
	}

	// 4. Test Log Level GET & PUT
	{
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/console-logs/level", nil)
		h.GetConsoleLogLevel(c)

		if rec.Code != http.StatusOK {
			t.Fatalf("GetConsoleLogLevel status = %d, want 200", rec.Code)
		}

		// Change level to debug
		putBody := bytes.NewBufferString(`{"level": "debug"}`)
		recPut := httptest.NewRecorder()
		cPut, _ := gin.CreateTestContext(recPut)
		cPut.Request = httptest.NewRequest(http.MethodPut, "/console-logs/level", putBody)
		cPut.Request.Header.Set("Content-Type", "application/json")
		h.PutConsoleLogLevel(cPut)

		if recPut.Code != http.StatusOK {
			t.Fatalf("PutConsoleLogLevel status = %d, want 200, body: %s", recPut.Code, recPut.Body.String())
		}
		var lvlResp struct {
			Success bool   `json:"success"`
			Level   string `json:"level"`
		}
		if err := json.Unmarshal(recPut.Body.Bytes(), &lvlResp); err != nil {
			t.Fatalf("failed to decode put level response: %v", err)
		}
		if lvlResp.Level != "debug" {
			t.Fatalf("expected level debug, got %s", lvlResp.Level)
		}

		// Invalid level should return 400
		badBody := bytes.NewBufferString(`{"level": "non_existent_level"}`)
		recBad := httptest.NewRecorder()
		cBad, _ := gin.CreateTestContext(recBad)
		cBad.Request = httptest.NewRequest(http.MethodPut, "/console-logs/level", badBody)
		cBad.Request.Header.Set("Content-Type", "application/json")
		h.PutConsoleLogLevel(cBad)

		if recBad.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for bad log level, got %d", recBad.Code)
		}
	}
}

func TestStreamConsoleLogsInitialBuffer(t *testing.T) {
	logging.RegisterConsoleLogHook()
	logging.ClearConsoleBuffer()

	h := NewHandlerWithoutConfigFilePath(&config.Config{}, nil)

	log.WithField("request_id", "stream-test-1").Info("Line for streaming test")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	// Context with fast cancellation so stream exits cleanly
	req := httptest.NewRequest(http.MethodGet, "/console-logs/stream", nil)
	ctx, cancel := req.Context(), func() {}
	c.Request = req.WithContext(ctx)

	// Run stream in goroutine and abort request context after reading init event
	go func() {
		time.Sleep(50 * time.Millisecond)
		c.Request.Context().Done()
	}()

	done := make(chan struct{})
	go func() {
		h.StreamConsoleLogs(c)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		cancel()
	}

	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte("stream-test-1")) {
		t.Fatalf("expected stream to contain initial buffer with stream-test-1, got body: %s", body)
	}
}
