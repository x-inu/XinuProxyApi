package auth

import (
	"time"

	xinugateauth "github.com/x-inu/XinuGate/v7/sdk/xinugate/auth"
)

func init() {
	registerRefreshLead("codex", func() Authenticator { return NewCodexAuthenticator() })
	registerRefreshLead("claude", func() Authenticator { return NewClaudeAuthenticator() })
	registerRefreshLead("antigravity", func() Authenticator { return NewAntigravityAuthenticator() })
	registerRefreshLead("kimi", func() Authenticator { return NewKimiAuthenticator() })
	registerRefreshLead("kimi-ai", func() Authenticator { return NewKimiAIAuthenticator() })
	registerRefreshLead("kimi.ai", func() Authenticator { return NewKimiAIDotAuthenticator() })
	registerRefreshLead("xai", func() Authenticator { return NewXAIAuthenticator() })
	registerRefreshLead("devin", func() Authenticator { return NewDevinAuthenticator() })
	registerRefreshLead("meta", func() Authenticator { return NewMetaAuthenticator() })
}

func registerRefreshLead(provider string, factory func() Authenticator) {
	xinugateauth.RegisterRefreshLeadProvider(provider, func() *time.Duration {
		if factory == nil {
			return nil
		}
		auth := factory()
		if auth == nil {
			return nil
		}
		return auth.RefreshLead()
	})
}
