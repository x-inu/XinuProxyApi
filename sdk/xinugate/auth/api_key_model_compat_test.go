package auth

import (
	"testing"

	internalconfig "github.com/x-inu/XinuGate/v7/internal/config"
	xinugateexecutor "github.com/x-inu/XinuGate/v7/sdk/xinugate/executor"
)

func TestResolvedAPIKeyModelInfoPropagatesIsCompat(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	manager.SetConfig(&internalconfig.Config{ClaudeKey: []internalconfig.ClaudeKey{{
		APIKey: "compat-key",
		Prefix: "tenant",
		Models: []internalconfig.ClaudeModel{{
			Name:     "deepseek-upstream",
			Alias:    "deepseek-alias",
			IsCompat: true,
		}},
	}}})
	auth := configuredCapabilityTestAuth("compat-auth", "compat-key")
	registerCapabilityTestAuth(t, manager, auth)

	req := manager.attachResolvedAPIKeyModelInfo(xinugateexecutor.Request{}, auth, "tenant/deepseek-alias", "deepseek-upstream")
	info, ok := ResolvedAPIKeyModelInfo(req)
	if !ok || info == nil || !info.IsCompat {
		t.Fatalf("ResolvedAPIKeyModelInfo() = (%+v, %t), want IsCompat=true", info, ok)
	}
}
