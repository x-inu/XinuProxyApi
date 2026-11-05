package management

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/logging"
)

const consolePingInterval = 25 * time.Second

// GetConsoleLogs returns in-memory buffered console log entries.
func (h *Handler) GetConsoleLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"logs":    logging.GetConsoleBuffer(),
	})
}

// DeleteConsoleLogs clears all buffered console logs and notifies subscribers.
func (h *Handler) DeleteConsoleLogs(c *gin.Context) {
	logging.ClearConsoleBuffer()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetConsoleLogLevel returns the current runtime server log level.
func (h *Handler) GetConsoleLogLevel(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"level":   logging.GetRuntimeLogLevel(),
	})
}

// PutConsoleLogLevel updates the server log level at runtime without restarting.
// Body: {"level": "debug"|"info"|"warn"|"error"}
func (h *Handler) PutConsoleLogLevel(c *gin.Context) {
	var req struct {
		Level string `json:"level"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Level) == "" {
		// Fallback to query param or form value if JSON body is missing
		req.Level = c.Query("level")
	}

	if strings.TrimSpace(req.Level) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing log level parameter"})
		return
	}

	newLevel, err := logging.SetRuntimeLogLevel(req.Level)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"level":   newLevel,
	})
}

// StreamConsoleLogs streams real-time console log entries via Server-Sent Events (SSE).
func (h *Handler) StreamConsoleLogs(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	send := func(ev logging.ConsoleEvent) bool {
		data, err := json.Marshal(ev)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	// 1. Send initial buffered entries
	buffered := logging.GetConsoleBuffer()
	if !send(logging.ConsoleEvent{Type: "init", Entries: buffered}) {
		return
	}

	// 2. Subscribe to live stream
	ch, cancel := logging.SubscribeConsoleLogs()
	defer cancel()

	ping := time.NewTicker(consolePingInterval)
	defer ping.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if !send(ev) {
				return
			}
		case <-ping.C:
			if _, err := io.WriteString(c.Writer, ": ping\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}
