package management

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/semanticcache"
)

// semanticCacheStatsResponse is the payload returned by GetSemanticCacheStats.
type semanticCacheStatsResponse struct {
	Enabled bool `json:"enabled"`
	semanticcache.Stats
}

// GetSemanticCacheStats returns overall semantic cache metrics.
func (h *Handler) GetSemanticCacheStats(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}
	cache := semanticcache.Global()
	if cache == nil {
		c.JSON(http.StatusOK, semanticCacheStatsResponse{
			Enabled: false,
			Stats:   semanticcache.Stats{HitRate: "0.0"},
		})
		return
	}
	c.JSON(http.StatusOK, semanticCacheStatsResponse{
		Enabled: cache.Enabled(),
		Stats:   cache.Stats(),
	})
}

// ListSemanticCacheEntries returns paginated cached entries metadata.
func (h *Handler) ListSemanticCacheEntries(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}
	cache := semanticcache.Global()
	if cache == nil {
		c.JSON(http.StatusOK, gin.H{"entries": []semanticcache.EntryMeta{}, "total": 0})
		return
	}

	page := parsePositiveInt(c.DefaultQuery("page", "1"), 1)
	limit := parsePositiveInt(c.DefaultQuery("limit", "20"), 20)
	if limit > 200 {
		limit = 200
	}
	search := strings.TrimSpace(c.Query("search"))
	model := strings.TrimSpace(c.Query("model"))
	sortBy := strings.TrimSpace(c.DefaultQuery("sort_by", "created_at"))
	sortOrder := strings.ToLower(strings.TrimSpace(c.DefaultQuery("sort_order", "desc")))

	entries, total := cache.ListEntries(page, limit, search, model, sortBy, sortOrder)
	c.JSON(http.StatusOK, gin.H{
		"entries": entries,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// DeleteSemanticCacheEntry removes a single cached entry by id.
func (h *Handler) DeleteSemanticCacheEntry(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}
	cache := semanticcache.Global()
	if cache == nil {
		c.JSON(http.StatusOK, gin.H{"status": "disabled"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	// The id is the full cache key; allow URL-encoded colon-free hashes.
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing entry id"})
		return
	}
	if !cache.DeleteEntry(c.Request.Context(), id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// InvalidateSemanticCacheByModel removes all entries for a model.
func (h *Handler) InvalidateSemanticCacheByModel(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}
	cache := semanticcache.Global()
	if cache == nil {
		c.JSON(http.StatusOK, gin.H{"status": "disabled"})
		return
	}
	model := strings.TrimSpace(c.Query("model"))
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing model"})
		return
	}
	n := cache.InvalidateByModel(c.Request.Context(), model)
	c.JSON(http.StatusOK, gin.H{"status": "invalidated", "removed": n})
}

// ClearSemanticCache flushes all cached entries.
func (h *Handler) ClearSemanticCache(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}
	cache := semanticcache.Global()
	if cache == nil {
		c.JSON(http.StatusOK, gin.H{"status": "disabled"})
		return
	}
	cache.Clear()
	c.JSON(http.StatusOK, gin.H{"status": "cleared"})
}

func parsePositiveInt(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
