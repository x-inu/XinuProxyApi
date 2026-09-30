package usagehistory

import (
	"context"
	"strings"

	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

// plugin implements coreusage.Plugin and folds each usage record into the
// global rollup store when history is enabled.
type plugin struct{}

func init() {
	coreusage.RegisterPlugin(&plugin{})
}

// HandleUsage is invoked for every completed request. Records are ignored when
// the history store is not configured.
func (p *plugin) HandleUsage(_ context.Context, record coreusage.Record) {
	store := Global()
	if store == nil {
		return
	}

	detail := coreusage.EnsureTokenBreakdownForProvider(record.Detail, record.Provider, record.ExecutorType)
	breakdown := detail.TokenBreakdown

	uncachedInput := breakdown.Input.UncachedTokens
	cacheRead := breakdown.Input.CacheReadTokens
	cacheWrite := breakdown.Input.CacheWriteTokens
	outputTokens := breakdown.Output.TotalTokens

	// Fall back to the flat counters when the v2 breakdown is unavailable.
	if breakdown.Input.TotalTokens == 0 && detail.InputTokens > 0 {
		cacheRead = detail.CacheReadTokens
		cacheWrite = detail.CacheCreationTokens
		uncachedInput = detail.InputTokens - cacheRead - cacheWrite
		if uncachedInput < 0 {
			uncachedInput = detail.InputTokens
			cacheRead = 0
			cacheWrite = 0
		}
	}
	if outputTokens == 0 {
		outputTokens = detail.OutputTokens
	}

	store.Add(Sample{
		Timestamp:     record.RequestedAt,
		Provider:      strings.TrimSpace(record.Provider),
		Model:         strings.TrimSpace(record.Model),
		Failed:        record.Failed,
		UncachedInput: uncachedInput,
		CacheRead:     cacheRead,
		CacheWrite:    cacheWrite,
		CachedTokens:  detail.CachedTokens,
		OutputTokens:  outputTokens,
		TotalTokens:   detail.TotalTokens,
	})
}
