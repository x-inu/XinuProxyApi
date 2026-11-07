package chat_completions

import (
	. "github.com/x-inu/XinuGate/v7/internal/constant"
	"github.com/x-inu/XinuGate/v7/internal/interfaces"
	"github.com/x-inu/XinuGate/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		OpenAI,
		Antigravity,
		ConvertOpenAIRequestToAntigravity,
		interfaces.TranslateResponse{
			Stream:    ConvertAntigravityResponseToOpenAI,
			NonStream: ConvertAntigravityResponseToOpenAINonStream,
		},
	)
}
