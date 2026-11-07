package interactions

import (
	. "github.com/x-inu/XinuProxyApi/v7/internal/constant"
	"github.com/x-inu/XinuProxyApi/v7/internal/interfaces"
	"github.com/x-inu/XinuProxyApi/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		Interactions,
		Interactions,
		ConvertInteractionsRequestToInteractions,
		interfaces.TranslateResponse{
			Stream:    ConvertInteractionsResponsePassthrough,
			NonStream: ConvertInteractionsResponsePassthroughNonStream,
		},
	)
	translator.Register(
		Interactions,
		Gemini,
		ConvertInteractionsRequestToGemini,
		interfaces.TranslateResponse{
			Stream:    ConvertGeminiResponseToInteractions,
			NonStream: ConvertGeminiResponseToInteractionsNonStream,
		},
	)
	translator.Register(
		Gemini,
		Interactions,
		ConvertGeminiRequestToInteractions,
		interfaces.TranslateResponse{
			Stream:    ConvertInteractionsResponseToGemini,
			NonStream: ConvertInteractionsResponseToGeminiNonStream,
		},
	)
}
