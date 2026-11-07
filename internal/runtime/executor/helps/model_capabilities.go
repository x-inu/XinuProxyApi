package helps

import (
	"github.com/x-inu/XinuGate/v7/internal/thinking"
	xinugateauth "github.com/x-inu/XinuGate/v7/sdk/xinugate/auth"
	xinugateexecutor "github.com/x-inu/XinuGate/v7/sdk/xinugate/executor"
)

// APIKeyModelIsCompat reports whether the selected API-key model enables
// compatibility handling for Claude thinking blocks.
func APIKeyModelIsCompat(req xinugateexecutor.Request) bool {
	modelInfo, ok := xinugateauth.ResolvedAPIKeyModelInfo(req)
	return ok && modelInfo != nil && modelInfo.IsCompat
}

// ApplyRequestThinking preserves the registry lookup path unless the auth
// manager bound authoritative model capabilities to this execution attempt.
func ApplyRequestThinking(body []byte, req xinugateexecutor.Request, opts xinugateexecutor.Options, fromFormat, toFormat, provider string, normalizedUpdatesChanged ...bool) ([]byte, error) {
	originalSource := opts.OriginalRequest
	if len(originalSource) == 0 {
		originalSource = req.Payload
	}
	source := req.Payload
	if len(source) == 0 {
		source = opts.OriginalRequest
	}
	summaryConfig := translatedRequestSummaryConfig(body, req.Payload, originalSource, req.Model, fromFormat, toFormat)
	if modelInfo, ok := xinugateauth.ResolvedModelInfo(req); ok {
		return thinking.ApplyThinkingWithModelInfoAndSummary(body, source, req.Model, fromFormat, toFormat, provider, modelInfo, summaryConfig, normalizedUpdatesChanged...)
	}
	return thinking.ApplyThinkingWithSourceAndSummary(body, source, req.Model, fromFormat, toFormat, provider, summaryConfig, normalizedUpdatesChanged...)
}
