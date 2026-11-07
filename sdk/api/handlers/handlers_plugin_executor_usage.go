package handlers

import (
	"github.com/x-inu/XinuGate/v7/internal/runtime/executor/helps"
	"github.com/x-inu/XinuGate/v7/sdk/xinugate/usage"
)

func parsePluginExecutorResponseUsage(protocol string, payload []byte) usage.Detail {
	return helps.ParsePluginExecutorResponseUsage(protocol, payload)
}

func observePluginExecutorStreamUsage(protocol string, payload []byte, buffer *helps.StreamUsageBuffer) {
	helps.ObservePluginExecutorStreamUsage(protocol, payload, buffer)
}
