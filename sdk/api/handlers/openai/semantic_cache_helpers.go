package openai

import "github.com/router-for-me/CLIProxyAPI/v7/sdk/config"

// cacheEnabled reports whether the semantic cache is explicitly enabled.
func cacheEnabled(cfg *config.SDKConfig) bool {
	if cfg == nil {
		return false
	}
	return cfg.SemanticCache.IsEnabled()
}
