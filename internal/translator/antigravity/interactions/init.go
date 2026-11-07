package interactions

import (
	. "github.com/x-inu/XinuGate/v7/internal/constant"
	"github.com/x-inu/XinuGate/v7/internal/interfaces"
	"github.com/x-inu/XinuGate/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		Interactions,
		Antigravity,
		ConvertInteractionsRequestToAntigravity,
		interfaces.TranslateResponse{
			Stream:    ConvertAntigravityResponseToInteractions,
			NonStream: ConvertAntigravityResponseToInteractionsNonStream,
		},
	)
}
