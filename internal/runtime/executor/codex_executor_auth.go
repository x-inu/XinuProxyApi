package executor

import (
	"context"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	codexauth "github.com/x-inu/XinuGate/v7/internal/auth/codex"
	"github.com/x-inu/XinuGate/v7/internal/config"
	"github.com/x-inu/XinuGate/v7/internal/runtime/executor/helps"
	xinugateauth "github.com/x-inu/XinuGate/v7/sdk/xinugate/auth"
	xinugateexecutor "github.com/x-inu/XinuGate/v7/sdk/xinugate/executor"
)

func (e *CodexExecutor) Refresh(ctx context.Context, auth *xinugateauth.Auth) (*xinugateauth.Auth, error) {
	log.Debugf("codex executor: refresh called")
	if refreshed, handled, err := helps.RefreshAuthViaHome(ctx, e.cfg, auth); handled {
		return refreshed, err
	}
	if auth == nil {
		return nil, statusErr{code: 500, msg: "codex executor: auth is nil"}
	}
	var refreshToken string
	if auth.Metadata != nil {
		if v, ok := auth.Metadata["refresh_token"].(string); ok && v != "" {
			refreshToken = v
		}
	}
	if refreshToken == "" {
		return auth, nil
	}
	svc := codexauth.NewCodexAuthWithProxyURL(e.cfg, auth.ProxyURL)
	td, err := svc.RefreshTokensWithRetry(ctx, refreshToken, 3)
	if err != nil {
		return nil, err
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	auth.Metadata["id_token"] = td.IDToken
	auth.Metadata["access_token"] = td.AccessToken
	if td.RefreshToken != "" {
		auth.Metadata["refresh_token"] = td.RefreshToken
	}
	if td.AccountID != "" {
		auth.Metadata["account_id"] = td.AccountID
	}
	auth.Metadata["email"] = td.Email
	// Use unified key in files
	auth.Metadata["expired"] = td.Expire
	auth.Metadata["type"] = "codex"
	now := time.Now().Format(time.RFC3339)
	auth.Metadata["last_refresh"] = now
	return auth, nil
}

func codexCreds(a *xinugateauth.Auth) (apiKey, baseURL string) {
	if a == nil {
		return "", ""
	}
	if a.Attributes != nil {
		apiKey = a.Attributes["api_key"]
		baseURL = a.Attributes["base_url"]
	}
	if apiKey == "" && a.Metadata != nil {
		if v, ok := a.Metadata["access_token"].(string); ok {
			apiKey = v
		}
	}
	return
}

func (e *CodexExecutor) resolveCodexConfig(auth *xinugateauth.Auth) *config.CodexKey {
	if e == nil {
		return nil
	}
	return resolveCodexKeyConfig(e.cfg, auth)
}

func resolveCodexKeyConfig(cfg *config.Config, auth *xinugateauth.Auth) *config.CodexKey {
	if auth == nil || cfg == nil {
		return nil
	}
	var attrKey, attrBase string
	if auth.Attributes != nil {
		attrKey = strings.TrimSpace(auth.Attributes["api_key"])
		attrBase = strings.TrimSpace(auth.Attributes["base_url"])
		if index, errIndex := strconv.Atoi(strings.TrimSpace(auth.Attributes[xinugateauth.AttributeConfigIndex])); errIndex == nil && index >= 0 && index < len(cfg.CodexKey) {
			entry := &cfg.CodexKey[index]
			cfgKey := strings.TrimSpace(entry.APIKey)
			cfgBase := strings.TrimSpace(entry.BaseURL)
			if (attrKey == "" || strings.EqualFold(cfgKey, attrKey)) && (attrBase == "" || strings.EqualFold(cfgBase, attrBase)) {
				return entry
			}
		}
	}
	for i := range cfg.CodexKey {
		entry := &cfg.CodexKey[i]
		cfgKey := strings.TrimSpace(entry.APIKey)
		cfgBase := strings.TrimSpace(entry.BaseURL)
		if attrKey != "" && attrBase != "" {
			if strings.EqualFold(cfgKey, attrKey) && strings.EqualFold(cfgBase, attrBase) {
				return entry
			}
			continue
		}
		if attrKey != "" && strings.EqualFold(cfgKey, attrKey) {
			if cfgBase == "" || strings.EqualFold(cfgBase, attrBase) {
				return entry
			}
		}
		if attrKey == "" && attrBase != "" && strings.EqualFold(cfgBase, attrBase) {
			return entry
		}
	}
	if attrKey != "" {
		for i := range cfg.CodexKey {
			entry := &cfg.CodexKey[i]
			if strings.EqualFold(strings.TrimSpace(entry.APIKey), attrKey) {
				return entry
			}
		}
	}
	return nil
}

func (e *CodexExecutor) resolveCodexModelIsCompat(auth *xinugateauth.Auth, req xinugateexecutor.Request, baseModel string) bool {
	if modelInfo, ok := xinugateauth.ResolvedModelInfo(req); ok && modelInfo != nil {
		return modelInfo.IsCompat
	}
	entry := e.resolveCodexConfig(auth)
	if entry != nil && len(entry.Models) > 0 {
		requested := strings.TrimSpace(req.Model)
		target := strings.TrimSpace(baseModel)
		for i := range entry.Models {
			name := strings.TrimSpace(entry.Models[i].Name)
			alias := strings.TrimSpace(entry.Models[i].Alias)
			if (target != "" && (strings.EqualFold(name, target) || strings.EqualFold(alias, target))) ||
				(requested != "" && (strings.EqualFold(name, requested) || strings.EqualFold(alias, requested))) {
				return entry.Models[i].IsCompat
			}
		}
		return false
	}
	if xinugateauth.CodexAPIKeyModelIsCompat(e.cfg, auth, baseModel) || xinugateauth.CodexAPIKeyModelIsCompat(e.cfg, auth, req.Model) {
		return true
	}
	return false
}
