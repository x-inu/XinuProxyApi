package helps

import (
	"context"

	"github.com/x-inu/XinuGate/v7/internal/logging"
	"github.com/x-inu/XinuGate/v7/internal/util"
	xinugateauth "github.com/x-inu/XinuGate/v7/sdk/xinugate/auth"
	xinugateexecutor "github.com/x-inu/XinuGate/v7/sdk/xinugate/executor"
)

// EnsureSessionContext ensures that ctx carries the internal session identity
// for $XG-SESSION-ID expansion in custom headers.
func EnsureSessionContext(ctx context.Context, opts xinugateexecutor.Options, payload []byte) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if id := util.SessionIDFromContext(ctx); id != "" {
		return ctx
	}
	if util.HasExplicitSessionID(ctx) {
		return ctx
	}
	if meta := logging.GetClientRequestMetadata(ctx); meta.SessionID != "" {
		return util.WithSessionID(ctx, meta.SessionID)
	}
	evalPayload := opts.OriginalRequest
	if len(evalPayload) == 0 {
		evalPayload = payload
	}
	canonical := xinugateauth.CanonicalSessionID(opts.Headers, evalPayload, opts.Metadata)
	if canonical != "" {
		return util.WithSessionID(ctx, canonical)
	}
	return ctx
}
