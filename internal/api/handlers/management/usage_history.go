package management

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usagehistory"
)

// usageHistoryModelRow aggregates totals for a single model.
type usageHistoryModelRow struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Requests     int64   `json:"requests"`
	Failed       int64   `json:"failed"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CachedTokens int64   `json:"cached_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// usageHistoryTimePoint aggregates totals for a single hour across all models.
type usageHistoryTimePoint struct {
	Hour         int64   `json:"hour"`
	Requests     int64   `json:"requests"`
	Failed       int64   `json:"failed"`
	TotalTokens  int64   `json:"total_tokens"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

type usageHistoryTotals struct {
	Requests     int64   `json:"requests"`
	Failed       int64   `json:"failed"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CachedTokens int64   `json:"cached_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

type usageHistoryResponse struct {
	Enabled  bool                    `json:"enabled"`
	Totals   usageHistoryTotals      `json:"totals"`
	ByModel  []usageHistoryModelRow  `json:"by_model"`
	Timeline []usageHistoryTimePoint `json:"timeline"`
}

// GetUsageHistory returns aggregated token usage and estimated cost, grouped by
// model and by hour. When the history store is disabled it returns an empty,
// non-error payload with enabled=false.
func (h *Handler) GetUsageHistory(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}

	store := usagehistory.Global()
	if store == nil {
		c.JSON(http.StatusOK, usageHistoryResponse{
			Enabled:  false,
			ByModel:  []usageHistoryModelRow{},
			Timeline: []usageHistoryTimePoint{},
		})
		return
	}

	buckets := store.Snapshot()

	modelMap := make(map[string]*usageHistoryModelRow)
	hourMap := make(map[int64]*usageHistoryTimePoint)
	var totals usageHistoryTotals

	for _, b := range buckets {
		modelKey := b.Provider + "\x00" + b.Model
		row, ok := modelMap[modelKey]
		if !ok {
			row = &usageHistoryModelRow{Provider: b.Provider, Model: b.Model}
			modelMap[modelKey] = row
		}
		row.Requests += b.Requests
		row.Failed += b.Failed
		row.InputTokens += b.InputTokens
		row.OutputTokens += b.OutputTokens
		row.CachedTokens += b.CachedTokens
		row.TotalTokens += b.TotalTokens
		row.CostUSD += b.CostUSD

		point, ok := hourMap[b.Hour]
		if !ok {
			point = &usageHistoryTimePoint{Hour: b.Hour}
			hourMap[b.Hour] = point
		}
		point.Requests += b.Requests
		point.Failed += b.Failed
		point.TotalTokens += b.TotalTokens
		point.InputTokens += b.InputTokens
		point.OutputTokens += b.OutputTokens
		point.CostUSD += b.CostUSD

		totals.Requests += b.Requests
		totals.Failed += b.Failed
		totals.InputTokens += b.InputTokens
		totals.OutputTokens += b.OutputTokens
		totals.CachedTokens += b.CachedTokens
		totals.TotalTokens += b.TotalTokens
		totals.CostUSD += b.CostUSD
	}

	byModel := make([]usageHistoryModelRow, 0, len(modelMap))
	for _, row := range modelMap {
		byModel = append(byModel, *row)
	}
	sort.Slice(byModel, func(i, j int) bool {
		if byModel[i].TotalTokens != byModel[j].TotalTokens {
			return byModel[i].TotalTokens > byModel[j].TotalTokens
		}
		if byModel[i].Provider != byModel[j].Provider {
			return byModel[i].Provider < byModel[j].Provider
		}
		return byModel[i].Model < byModel[j].Model
	})

	timeline := make([]usageHistoryTimePoint, 0, len(hourMap))
	for _, point := range hourMap {
		timeline = append(timeline, *point)
	}
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Hour < timeline[j].Hour
	})

	c.JSON(http.StatusOK, usageHistoryResponse{
		Enabled:  true,
		Totals:   totals,
		ByModel:  byModel,
		Timeline: timeline,
	})
}

// DeleteUsageHistory clears all stored usage history.
func (h *Handler) DeleteUsageHistory(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}
	store := usagehistory.Global()
	if store == nil {
		c.JSON(http.StatusOK, gin.H{"status": "disabled"})
		return
	}
	if err := store.Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": strings.TrimSpace(err.Error())})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cleared"})
}
