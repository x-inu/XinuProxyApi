package helps

import (
	"strings"

	"github.com/x-inu/XinuGate/v7/internal/util"
	sdktranslator "github.com/x-inu/XinuGate/v7/sdk/translator"
	xinugateexecutor "github.com/x-inu/XinuGate/v7/sdk/xinugate/executor"
)

// IsNativeCodexRequest checks the client dialect for use inside Codex executors.
func IsNativeCodexRequest(body []byte, opts xinugateexecutor.Options) bool {
	for _, format := range []sdktranslator.Format{opts.SourceFormat, xinugateexecutor.ResponseFormatOrSource(opts)} {
		name := strings.TrimSpace(format.String())
		if !strings.EqualFold(name, sdktranslator.FormatCodex.String()) && !strings.EqualFold(name, sdktranslator.FormatOpenAIResponse.String()) {
			return false
		}
	}
	return util.IsCodexResponsesLiteRequest(body, opts.Headers)
}
