package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usagestats"
)

// GetLatencyStats returns in-memory latency percentiles (p50/p95/p99) and
// model health (failure rate) aggregated from completed requests since process
// start. The data is bounded (reservoir ~1024 samples per model) and resets on
// restart; it is additive telemetry and never affects request handling.
func (h *Handler) GetLatencyStats(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}
	tracker := usagestats.Global()
	if tracker == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"overall": usagestats.OverallStats{ErrorRate: "0.0"},
			"by_model": []usagestats.BucketStats{},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled":  true,
		"overall":  tracker.SnapshotOverall(),
		"by_model": tracker.Snapshot(),
	})
}
