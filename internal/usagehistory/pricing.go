package usagehistory

import "strings"

// ModelRate holds estimated pricing for a model, expressed in USD per one
// million tokens. These values are approximations used for cost estimation
// only; upstream providers may change prices at any time.
type ModelRate struct {
	InputPerM      float64
	OutputPerM     float64
	CacheReadPerM  float64
	CacheWritePerM float64
}

type ratePrefix struct {
	prefix string
	rate   ModelRate
}

// defaultRates maps lower-cased model-name prefixes to estimated pricing.
// Longer prefixes take precedence, so more specific model names win.
var defaultRates = []ratePrefix{
	// OpenAI / Codex
	{"gpt-5-mini", ModelRate{InputPerM: 0.25, OutputPerM: 2.0}},
	{"gpt-5-nano", ModelRate{InputPerM: 0.05, OutputPerM: 0.4}},
	{"gpt-5", ModelRate{InputPerM: 1.25, OutputPerM: 10.0}},
	{"gpt-4.1-mini", ModelRate{InputPerM: 0.4, OutputPerM: 1.6}},
	{"gpt-4.1-nano", ModelRate{InputPerM: 0.1, OutputPerM: 0.4}},
	{"gpt-4.1", ModelRate{InputPerM: 2.0, OutputPerM: 8.0}},
	{"gpt-4o-mini", ModelRate{InputPerM: 0.15, OutputPerM: 0.6}},
	{"gpt-4o", ModelRate{InputPerM: 2.5, OutputPerM: 10.0}},
	{"o4-mini", ModelRate{InputPerM: 1.1, OutputPerM: 4.4}},
	{"o3-mini", ModelRate{InputPerM: 1.1, OutputPerM: 4.4}},
	{"o3", ModelRate{InputPerM: 2.0, OutputPerM: 8.0}},
	{"o1-mini", ModelRate{InputPerM: 1.1, OutputPerM: 4.4}},
	{"o1", ModelRate{InputPerM: 15.0, OutputPerM: 60.0}},
	{"codex", ModelRate{InputPerM: 1.25, OutputPerM: 10.0}},

	// Anthropic / Claude
	{"claude-3-5-haiku", ModelRate{InputPerM: 0.8, OutputPerM: 4.0}},
	{"claude-3-5-sonnet", ModelRate{InputPerM: 3.0, OutputPerM: 15.0}},
	{"claude-3-opus", ModelRate{InputPerM: 15.0, OutputPerM: 75.0}},
	{"claude-3-haiku", ModelRate{InputPerM: 0.25, OutputPerM: 1.25}},
	{"claude-haiku-4", ModelRate{InputPerM: 1.0, OutputPerM: 5.0}},
	{"claude-sonnet-4", ModelRate{InputPerM: 3.0, OutputPerM: 15.0}},
	{"claude-opus-4", ModelRate{InputPerM: 15.0, OutputPerM: 75.0}},
	{"claude", ModelRate{InputPerM: 3.0, OutputPerM: 15.0}},

	// Google / Gemini
	{"gemini-3.7-flash", ModelRate{InputPerM: 0.75, OutputPerM: 3.75}},
	{"gemini-3.5-flash-lite", ModelRate{InputPerM: 0.1, OutputPerM: 0.4}},
	{"gemini-3.5-flash", ModelRate{InputPerM: 0.3, OutputPerM: 2.5}},
	{"gemini-3-flash", ModelRate{InputPerM: 0.3, OutputPerM: 2.5}},
	{"gemini-2.5-pro", ModelRate{InputPerM: 1.25, OutputPerM: 10.0}},
	{"gemini-2.5-flash-lite", ModelRate{InputPerM: 0.1, OutputPerM: 0.4}},
	{"gemini-2.5-flash", ModelRate{InputPerM: 0.3, OutputPerM: 2.5}},
	{"gemini-2.0-flash-lite", ModelRate{InputPerM: 0.075, OutputPerM: 0.3}},
	{"gemini-2.0-flash", ModelRate{InputPerM: 0.1, OutputPerM: 0.4}},
	{"gemini-1.5-pro", ModelRate{InputPerM: 1.25, OutputPerM: 5.0}},
	{"gemini-1.5-flash", ModelRate{InputPerM: 0.075, OutputPerM: 0.3}},
	{"gemini", ModelRate{InputPerM: 0.3, OutputPerM: 2.5}},

	// xAI / Grok
	{"grok-4", ModelRate{InputPerM: 3.0, OutputPerM: 15.0}},
	{"grok-3-mini", ModelRate{InputPerM: 0.3, OutputPerM: 0.5}},
	{"grok-3", ModelRate{InputPerM: 3.0, OutputPerM: 15.0}},
	{"grok", ModelRate{InputPerM: 3.0, OutputPerM: 15.0}},

	// Moonshot / Kimi
	{"kimi", ModelRate{InputPerM: 0.6, OutputPerM: 2.5}},
	{"moonshot", ModelRate{InputPerM: 0.6, OutputPerM: 2.5}},

	// DeepSeek
	{"deepseek", ModelRate{InputPerM: 0.27, OutputPerM: 1.1}},

	// Qwen
	{"qwen", ModelRate{InputPerM: 0.4, OutputPerM: 1.2}},
}

// rateFor returns the estimated pricing for a model, selecting the longest
// matching known prefix. Unknown models return a zero rate so their cost is 0.
func rateFor(model string) (ModelRate, bool) {
	name := strings.ToLower(strings.TrimSpace(model))
	if name == "" {
		return ModelRate{}, false
	}
	best := -1
	var bestRate ModelRate
	for _, entry := range defaultRates {
		if strings.HasPrefix(name, entry.prefix) && len(entry.prefix) > best {
			best = len(entry.prefix)
			bestRate = entry.rate
		}
	}
	if best < 0 {
		return ModelRate{}, false
	}
	return bestRate, true
}

// RateFor returns the estimated pricing for a model, selecting the longest
// matching known prefix. Unknown models return a zero rate so their cost is 0.
func RateFor(model string) (ModelRate, bool) {
	return rateFor(model)
}

// estimateCost computes an estimated USD cost from non-overlapping token counts.
// cacheRead defaults to 10% of the input rate and cacheWrite to the input rate
// when the model does not define explicit cache pricing.
func estimateCost(rate ModelRate, uncachedInput, cacheRead, cacheWrite, output int64) float64 {
	const perMillion = 1_000_000.0
	cacheReadRate := rate.CacheReadPerM
	if cacheReadRate == 0 {
		cacheReadRate = rate.InputPerM * 0.1
	}
	cacheWriteRate := rate.CacheWritePerM
	if cacheWriteRate == 0 {
		cacheWriteRate = rate.InputPerM
	}
	cost := float64(uncachedInput)/perMillion*rate.InputPerM +
		float64(cacheRead)/perMillion*cacheReadRate +
		float64(cacheWrite)/perMillion*cacheWriteRate +
		float64(output)/perMillion*rate.OutputPerM
	if cost < 0 {
		return 0
	}
	return cost
}
