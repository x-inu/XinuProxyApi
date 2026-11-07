package auth

import (
	"context"
	"testing"

	xinugateexecutor "github.com/x-inu/XinuGate/v7/sdk/xinugate/executor"
	coreusage "github.com/x-inu/XinuGate/v7/sdk/xinugate/usage"
)

func TestContextWithRequestedModelAliasIncludesStream(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "stream_false", true: "stream_true"}[stream], func(t *testing.T) {
			ctx := contextWithRequestedModelAlias(context.Background(), xinugateexecutor.Options{
				Stream: stream,
			}, "fallback-model")

			if got := coreusage.StreamFromContext(ctx); got != stream {
				t.Fatalf("stream = %v, want %v", got, stream)
			}
		})
	}
}

func TestContextWithRequestedModelAliasIncludesReasoningEffort(t *testing.T) {
	ctx := contextWithRequestedModelAlias(context.Background(), xinugateexecutor.Options{
		Metadata: map[string]any{
			xinugateexecutor.RequestedModelMetadataKey:  "client-model",
			xinugateexecutor.ReasoningEffortMetadataKey: "medium",
			xinugateexecutor.ServiceTierMetadataKey:     "auto",
			xinugateexecutor.GenerateMetadataKey:        false,
		},
	}, "fallback-model")

	if got := coreusage.RequestedModelAliasFromContext(ctx); got != "client-model" {
		t.Fatalf("requested model alias = %q, want %q", got, "client-model")
	}
	if got := coreusage.ReasoningEffortFromContext(ctx); got != "medium" {
		t.Fatalf("reasoning effort = %q, want %q", got, "medium")
	}
	gotServiceTier := coreusage.ServiceTierFromContext(ctx)
	if gotServiceTier != "auto" {
		t.Fatalf("service tier = %q, want %q", gotServiceTier, "auto")
	}
	if got := coreusage.GenerateFromContext(ctx); got {
		t.Fatalf("generate = %v, want false", got)
	}
}

func TestContextWithRequestedModelAliasDefaultsGenerateTrue(t *testing.T) {
	ctx := contextWithRequestedModelAlias(context.Background(), xinugateexecutor.Options{
		Metadata: map[string]any{
			xinugateexecutor.RequestedModelMetadataKey: "client-model",
		},
	}, "fallback-model")

	if got := coreusage.GenerateFromContext(ctx); !got {
		t.Fatalf("generate = %v, want true", got)
	}
}

func TestContextWithRequestedModelAliasPreservesExistingGenerateFalse(t *testing.T) {
	ctx := coreusage.WithGenerate(context.Background(), false)
	ctx = contextWithRequestedModelAlias(ctx, xinugateexecutor.Options{
		Metadata: map[string]any{
			xinugateexecutor.RequestedModelMetadataKey: "client-model",
		},
	}, "fallback-model")

	if got := coreusage.GenerateFromContext(ctx); got {
		t.Fatalf("generate = %v, want false", got)
	}
}
