package interactions

import (
	. "github.com/x-inu/XinuProxyApi/v7/internal/constant"
	"github.com/x-inu/XinuProxyApi/v7/internal/interfaces"
	"github.com/x-inu/XinuProxyApi/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		Interactions,
		Codex,
		ConvertInteractionsRequestToCodex,
		interfaces.TranslateResponse{
			Stream:    ConvertCodexResponseToInteractions,
			NonStream: ConvertCodexResponseToInteractionsNonStream,
		},
	)
}
