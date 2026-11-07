package chat_completions

import (
	. "github.com/x-inu/XinuProxyApi/v7/internal/constant"
	"github.com/x-inu/XinuProxyApi/v7/internal/interfaces"
	"github.com/x-inu/XinuProxyApi/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		OpenAI,
		Claude,
		ConvertOpenAIRequestToClaude,
		interfaces.TranslateResponse{
			Stream:    ConvertClaudeResponseToOpenAI,
			NonStream: ConvertClaudeResponseToOpenAINonStream,
		},
	)
}
