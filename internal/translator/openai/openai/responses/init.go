package responses

import (
	. "github.com/x-inu/XinuGate/v7/internal/constant"
	"github.com/x-inu/XinuGate/v7/internal/interfaces"
	"github.com/x-inu/XinuGate/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		OpenaiResponse,
		OpenAI,
		ConvertOpenAIResponsesRequestToOpenAIChatCompletions,
		interfaces.TranslateResponse{
			Stream:    ConvertOpenAIChatCompletionsResponseToOpenAIResponses,
			NonStream: ConvertOpenAIChatCompletionsResponseToOpenAIResponsesNonStream,
		},
	)
}
