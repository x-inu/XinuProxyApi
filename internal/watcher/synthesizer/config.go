package synthesizer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/constant"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/util"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/watcher/diff"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

// ConfigSynthesizer generates Auth entries from configuration API keys.
// It handles Gemini, Interactions, Claude, Codex, xAI, OpenAI-compat, and Vertex-compat providers.
type ConfigSynthesizer struct{}

// NewConfigSynthesizer creates a new ConfigSynthesizer instance.
func NewConfigSynthesizer() *ConfigSynthesizer {
	return &ConfigSynthesizer{}
}

func addWeightToAttrs(weight *int, attrs map[string]string) {
	if weight == nil {
		return
	}
	normalized := *weight
	if normalized <= 0 {
		normalized = 0
	}
	attrs[coreauth.AttributeWeight] = strconv.Itoa(normalized)
}

// Synthesize generates Auth entries from config API keys.
func (s *ConfigSynthesizer) Synthesize(ctx *SynthesisContext) ([]*coreauth.Auth, error) {
	out := make([]*coreauth.Auth, 0, 32)
	if ctx == nil || ctx.Config == nil {
		return out, nil
	}
	if errValidate := ctx.Config.ValidateCredentialWeights(); errValidate != nil {
		return nil, fmt.Errorf("synthesize config API key auths: %w", errValidate)
	}

	// Gemini API Keys
	out = append(out, s.synthesizeGeminiKeys(ctx)...)
	// Native Interactions API Keys
	out = append(out, s.synthesizeInteractionsKeys(ctx)...)
	// Claude API Keys
	out = append(out, s.synthesizeClaudeKeys(ctx)...)
	// Codex API Keys
	out = append(out, s.synthesizeCodexKeys(ctx)...)
	// xAI API Keys
	out = append(out, s.synthesizeXAIKeys(ctx)...)
	// Meta API Keys
	out = append(out, s.synthesizeMetaKeys(ctx)...)
	// OpenAI-compat
	out = append(out, s.synthesizeOpenAICompat(ctx)...)
	// Vertex-compat
	out = append(out, s.synthesizeVertexCompat(ctx)...)

	return out, nil
}

// synthesizeGeminiKeys creates Auth entries for Gemini API keys.
func (s *ConfigSynthesizer) synthesizeGeminiKeys(ctx *SynthesisContext) []*coreauth.Auth {
	return s.synthesizeGeminiKeyEntries(ctx, ctx.Config.GeminiKey, "gemini:apikey", "gemini", "gemini-apikey", constant.Gemini)
}

// synthesizeInteractionsKeys creates Auth entries for native Interactions API keys.
func (s *ConfigSynthesizer) synthesizeInteractionsKeys(ctx *SynthesisContext) []*coreauth.Auth {
	return s.synthesizeGeminiKeyEntries(ctx, ctx.Config.InteractionsKey, "gemini-interactions:apikey", "interactions", "interactions-apikey", constant.GeminiInteractions)
}

func (s *ConfigSynthesizer) synthesizeGeminiKeyEntries(ctx *SynthesisContext, entries []config.GeminiKey, idKind, sourceName, label, provider string) []*coreauth.Auth {
	cfg := ctx.Config
	now := ctx.Now
	idGen := ctx.IDGenerator

	out := make([]*coreauth.Auth, 0, len(entries))
	for i := range entries {
		entry := entries[i]
		key := strings.TrimSpace(entry.APIKey)
		base := strings.TrimSpace(entry.BaseURL)
		if key == "" && base == "" {
			continue
		}
		prefix := strings.TrimSpace(entry.Prefix)
		proxyURL := strings.TrimSpace(entry.ProxyURL)
		id, token := idGen.Next(idKind, key, base, proxyURL, prefix, config.FormatSortedHeaders(entry.Headers))
		attrs := map[string]string{
			"source":       fmt.Sprintf("config:%s[%s]", sourceName, token),
			"config_index": strconv.Itoa(i),
		}
		if key != "" {
			attrs["api_key"] = key
		}
		metadata := map[string]any{}
		if entry.DisableCooling != nil {
			metadata["disable_cooling"] = *entry.DisableCooling
		}
		addRequestRetryToMetadata(entry.RequestRetry, metadata)
		addRequestScopedErrorsToMetadata(entry.RequestScopedErrors, metadata)
		if entry.Priority != 0 {
			attrs["priority"] = strconv.Itoa(entry.Priority)
		}
		addWeightToAttrs(entry.Weight, attrs)
		if base != "" {
			attrs["base_url"] = base
		}
		if hash := diff.ComputeGeminiModelsHash(entry.Models); hash != "" {
			attrs["models_hash"] = hash
		}
		addConfigHeadersToAttrs(entry.Headers, attrs)
		a := &coreauth.Auth{
			ID:         id,
			Provider:   provider,
			Label:      label,
			Prefix:     prefix,
			Status:     coreauth.StatusActive,
			ProxyURL:   proxyURL,
			Attributes: attrs,
			Metadata:   metadata,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		ApplyAuthExcludedModelsMeta(a, cfg, entry.ExcludedModels, "apikey")
		if len(a.Metadata) == 0 {
			a.Metadata = nil
		}
		out = append(out, a)
	}
	return out
}

// synthesizeClaudeKeys creates Auth entries for Claude API keys.
func (s *ConfigSynthesizer) synthesizeClaudeKeys(ctx *SynthesisContext) []*coreauth.Auth {
	cfg := ctx.Config
	out := make([]*coreauth.Auth, 0, len(cfg.ClaudeKey))
	for i := range cfg.ClaudeKey {
		ck := cfg.ClaudeKey[i]
		base := strings.TrimSpace(ck.BaseURL)
		prefix := strings.TrimSpace(ck.Prefix)
		if len(ck.APIKeyEntries) == 0 {
			out = append(out, s.synthesizeClaudeKeyEntry(ctx, i, ck, base, prefix, strings.TrimSpace(ck.APIKey), strings.TrimSpace(ck.ProxyURL), ck.Weight, "")...)
			continue
		}
		// Multi-key mode: each key becomes its own credential sharing the
		// entry's base URL, models, headers, and exclusion rules. The single
		// APIKey field is ignored so no duplicate credential is registered.
		for j := range ck.APIKeyEntries {
			entry := ck.APIKeyEntries[j]
			key := strings.TrimSpace(entry.APIKey)
			if key == "" {
				continue
			}
			proxyURL := strings.TrimSpace(entry.ProxyURL)
			if proxyURL == "" {
				proxyURL = strings.TrimSpace(ck.ProxyURL)
			}
			weight := entry.Weight
			if weight == nil {
				weight = ck.Weight
			}
			slot := strconv.Itoa(j)
			out = append(out, s.synthesizeClaudeKeyEntry(ctx, i, ck, base, prefix, key, proxyURL, weight, slot)...)
		}
	}
	return out
}

// synthesizeClaudeKeyEntry synthesizes one Claude auth credential. The slot
// parameter identifies the key inside a multi-key entry; it is empty for the
// legacy single-key form so existing credential IDs stay stable.
func (s *ConfigSynthesizer) synthesizeClaudeKeyEntry(
	ctx *SynthesisContext,
	index int,
	ck config.ClaudeKey,
	base string,
	prefix string,
	key string,
	proxyURL string,
	weight *int,
	slot string,
) []*coreauth.Auth {
	if key == "" && base == "" {
		return nil
	}
	cfg := ctx.Config
	now := ctx.Now
	idGen := ctx.IDGenerator
	idParts := []string{key, base, proxyURL, prefix, config.FormatSortedHeaders(ck.Headers)}
	if slot != "" {
		idParts = append(idParts, slot)
	}
	id, token := idGen.Next("claude:apikey", idParts...)
	attrs := map[string]string{
		"source":       fmt.Sprintf("config:claude[%s]", token),
		"config_index": strconv.Itoa(index),
	}
	if key != "" {
		attrs["api_key"] = key
	}
	if slot != "" {
		attrs["key_slot"] = slot
	}
	metadata := map[string]any{}
	if ck.DisableCooling != nil {
		metadata["disable_cooling"] = *ck.DisableCooling
	}
	addRequestRetryToMetadata(ck.RequestRetry, metadata)
	addRequestScopedErrorsToMetadata(ck.RequestScopedErrors, metadata)
	if ck.Priority != 0 {
		attrs["priority"] = strconv.Itoa(ck.Priority)
	}
	addWeightToAttrs(weight, attrs)
	if base != "" {
		attrs["base_url"] = base
	}
	if ck.RebuildMidSystemMessage {
		attrs["rebuild_mid_system_message"] = "true"
	}
	if profile := strings.ToLower(strings.TrimSpace(ck.FingerprintProfile)); profile != "" {
		attrs["fingerprint_profile"] = profile
	}
	if hash := diff.ComputeClaudeModelsHash(ck.Models); hash != "" {
		attrs["models_hash"] = hash
	}
	addConfigHeadersToAttrs(ck.Headers, attrs)
	a := &coreauth.Auth{
		ID:         id,
		Provider:   "claude",
		Label:      "claude-apikey",
		Prefix:     prefix,
		Status:     coreauth.StatusActive,
		ProxyURL:   proxyURL,
		Attributes: attrs,
		Metadata:   metadata,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	ApplyAuthExcludedModelsMeta(a, cfg, ck.ExcludedModels, "apikey")
	if len(a.Metadata) == 0 {
		a.Metadata = nil
	}
	return []*coreauth.Auth{a}
}

// synthesizeCodexKeys creates Auth entries for Codex API keys.
func (s *ConfigSynthesizer) synthesizeCodexKeys(ctx *SynthesisContext) []*coreauth.Auth {
	return s.synthesizeCodexStyleKeys(ctx, ctx.Config.CodexKey, "codex")
}

// synthesizeXAIKeys creates Auth entries for xAI API keys.
func (s *ConfigSynthesizer) synthesizeXAIKeys(ctx *SynthesisContext) []*coreauth.Auth {
	return s.synthesizeCodexStyleKeys(ctx, ctx.Config.XAIKey, "xai")
}

// synthesizeMetaKeys creates Auth entries for Meta API keys.
func (s *ConfigSynthesizer) synthesizeMetaKeys(ctx *SynthesisContext) []*coreauth.Auth {
	return s.synthesizeCodexStyleKeys(ctx, ctx.Config.MetaKey, "meta")
}

func (s *ConfigSynthesizer) synthesizeCodexStyleKeys(ctx *SynthesisContext, entries []config.CodexKey, provider string) []*coreauth.Auth {
	cfg := ctx.Config
	now := ctx.Now
	idGen := ctx.IDGenerator

	out := make([]*coreauth.Auth, 0, len(entries))
	for i := range entries {
		entry := entries[i]
		key := strings.TrimSpace(entry.APIKey)
		baseURL := strings.TrimSpace(entry.BaseURL)
		if key == "" && baseURL == "" {
			continue
		}
		prefix := strings.TrimSpace(entry.Prefix)
		proxyURL := strings.TrimSpace(entry.ProxyURL)
		id, token := idGen.Next(provider+":apikey", key, baseURL, proxyURL, prefix, config.FormatSortedHeaders(entry.Headers))
		attrs := map[string]string{
			"source":       fmt.Sprintf("config:%s[%s]", provider, token),
			"config_index": strconv.Itoa(i),
		}
		if key != "" {
			attrs["api_key"] = key
		}
		metadata := map[string]any{}
		if entry.DisableCooling != nil {
			metadata["disable_cooling"] = *entry.DisableCooling
		}
		addRequestRetryToMetadata(entry.RequestRetry, metadata)
		addRequestScopedErrorsToMetadata(entry.RequestScopedErrors, metadata)
		if entry.Priority != 0 {
			attrs["priority"] = strconv.Itoa(entry.Priority)
		}
		addWeightToAttrs(entry.Weight, attrs)
		if baseURL != "" {
			attrs["base_url"] = baseURL
		}
		if entry.Websockets {
			attrs["websockets"] = "true"
		}
		if provider == "codex" && entry.AlphaSearch {
			attrs[coreauth.AttributeCodexAlphaSearch] = "true"
		}
		if provider == "codex" && entry.DisableCodexCloaking != nil {
			attrs[coreauth.AttributeCodexDisableCloaking] = strconv.FormatBool(*entry.DisableCodexCloaking)
		}
		if hash := diff.ComputeCodexModelsHash(entry.Models); hash != "" {
			attrs["models_hash"] = hash
		}
		addConfigHeadersToAttrs(entry.Headers, attrs)
		a := &coreauth.Auth{
			ID:         id,
			Provider:   provider,
			Label:      provider + "-apikey",
			Prefix:     prefix,
			Status:     coreauth.StatusActive,
			ProxyURL:   strings.TrimSpace(entry.ProxyURL),
			Attributes: attrs,
			Metadata:   metadata,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		ApplyAuthExcludedModelsMeta(a, cfg, entry.ExcludedModels, "apikey")
		if len(a.Metadata) == 0 {
			a.Metadata = nil
		}
		out = append(out, a)
	}
	return out
}

// synthesizeOpenAICompat creates Auth entries for OpenAI-compatible providers.
func (s *ConfigSynthesizer) synthesizeOpenAICompat(ctx *SynthesisContext) []*coreauth.Auth {
	cfg := ctx.Config
	now := ctx.Now
	idGen := ctx.IDGenerator

	out := make([]*coreauth.Auth, 0)
	for i := range cfg.OpenAICompatibility {
		compat := &cfg.OpenAICompatibility[i]
		if compat.Disabled {
			continue
		}
		prefix := strings.TrimSpace(compat.Prefix)
		providerName := strings.ToLower(strings.TrimSpace(compat.Name))
		if providerName == "" {
			providerName = "openai-compatibility"
		}
		internalProviderKey := util.OpenAICompatibleProviderKey(providerName)
		base := strings.TrimSpace(compat.BaseURL)
		disableCooling := compat.DisableCooling

		// Handle new APIKeyEntries format (preferred)
		createdEntries := 0
		for j := range compat.APIKeyEntries {
			entry := &compat.APIKeyEntries[j]
			key := strings.TrimSpace(entry.APIKey)
			proxyURL := strings.TrimSpace(entry.ProxyURL)
			idKind := fmt.Sprintf("openai-compatibility:%s", providerName)
			id, token := idGen.Next(idKind, key, base, proxyURL)
			attrs := map[string]string{
				"source":                   fmt.Sprintf("config:%s[%s]", providerName, token),
				"base_url":                 base,
				"compat_name":              compat.Name,
				"provider_key":             internalProviderKey,
				"config_index":             strconv.Itoa(i),
				coreauth.AttributeAuthKind: coreauth.AuthKindAPIKey,
			}
			metadata := map[string]any{}
			if disableCooling != nil {
				metadata["disable_cooling"] = *disableCooling
			}
			addRequestRetryToMetadata(compat.RequestRetry, metadata)
			addRequestScopedErrorsToMetadata(compat.RequestScopedErrors, metadata)
			if compat.Priority != 0 {
				attrs["priority"] = strconv.Itoa(compat.Priority)
			}
			addWeightToAttrs(entry.Weight, attrs)
			if key != "" {
				attrs["api_key"] = key
			}
			if hash := diff.ComputeOpenAICompatModelsHash(compat.Models); hash != "" {
				attrs["models_hash"] = hash
			}
			addConfigHeadersToAttrs(compat.Headers, attrs)
			a := &coreauth.Auth{
				ID:         id,
				Provider:   internalProviderKey,
				Label:      compat.Name,
				Prefix:     prefix,
				Status:     coreauth.StatusActive,
				ProxyURL:   proxyURL,
				Attributes: attrs,
				Metadata:   metadata,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if len(a.Metadata) == 0 {
				a.Metadata = nil
			}
			out = append(out, a)
			createdEntries++
		}
		// Fallback: create entry without API key if no APIKeyEntries
		if createdEntries == 0 {
			idKind := fmt.Sprintf("openai-compatibility:%s", providerName)
			id, token := idGen.Next(idKind, base)
			attrs := map[string]string{
				"source":                   fmt.Sprintf("config:%s[%s]", providerName, token),
				"base_url":                 base,
				"compat_name":              compat.Name,
				"provider_key":             internalProviderKey,
				"config_index":             strconv.Itoa(i),
				coreauth.AttributeAuthKind: coreauth.AuthKindAPIKey,
			}
			metadata := map[string]any{}
			if disableCooling != nil {
				metadata["disable_cooling"] = *disableCooling
			}
			addRequestRetryToMetadata(compat.RequestRetry, metadata)
			addRequestScopedErrorsToMetadata(compat.RequestScopedErrors, metadata)
			if compat.Priority != 0 {
				attrs["priority"] = strconv.Itoa(compat.Priority)
			}
			if hash := diff.ComputeOpenAICompatModelsHash(compat.Models); hash != "" {
				attrs["models_hash"] = hash
			}
			addConfigHeadersToAttrs(compat.Headers, attrs)
			a := &coreauth.Auth{
				ID:         id,
				Provider:   internalProviderKey,
				Label:      compat.Name,
				Prefix:     prefix,
				Status:     coreauth.StatusActive,
				Attributes: attrs,
				Metadata:   metadata,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if len(a.Metadata) == 0 {
				a.Metadata = nil
			}
			out = append(out, a)
		}
	}
	return out
}

// synthesizeVertexCompat creates Auth entries for Vertex-compatible providers.
func (s *ConfigSynthesizer) synthesizeVertexCompat(ctx *SynthesisContext) []*coreauth.Auth {
	cfg := ctx.Config
	now := ctx.Now
	idGen := ctx.IDGenerator

	out := make([]*coreauth.Auth, 0, len(cfg.VertexCompatAPIKey))
	for i := range cfg.VertexCompatAPIKey {
		compat := &cfg.VertexCompatAPIKey[i]
		providerName := "vertex"
		base := strings.TrimSpace(compat.BaseURL)

		key := strings.TrimSpace(compat.APIKey)
		prefix := strings.TrimSpace(compat.Prefix)
		proxyURL := strings.TrimSpace(compat.ProxyURL)
		idKind := "vertex:apikey"
		id, token := idGen.Next(idKind, key, base, proxyURL)
		attrs := map[string]string{
			"source":       fmt.Sprintf("config:vertex-apikey[%s]", token),
			"base_url":     base,
			"provider_key": providerName,
			"config_index": strconv.Itoa(i),
		}
		if compat.Priority != 0 {
			attrs["priority"] = strconv.Itoa(compat.Priority)
		}
		addWeightToAttrs(compat.Weight, attrs)
		if key != "" {
			attrs["api_key"] = key
		}
		if hash := diff.ComputeVertexCompatModelsHash(compat.Models); hash != "" {
			attrs["models_hash"] = hash
		}
		addConfigHeadersToAttrs(compat.Headers, attrs)
		metadata := map[string]any{}
		if compat.DisableCooling != nil {
			metadata["disable_cooling"] = *compat.DisableCooling
		}
		addRequestRetryToMetadata(compat.RequestRetry, metadata)
		a := &coreauth.Auth{
			ID:         id,
			Provider:   providerName,
			Label:      "vertex-apikey",
			Prefix:     prefix,
			Status:     coreauth.StatusActive,
			ProxyURL:   proxyURL,
			Attributes: attrs,
			Metadata:   metadata,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		ApplyAuthExcludedModelsMeta(a, cfg, compat.ExcludedModels, "apikey")
		if len(a.Metadata) == 0 {
			a.Metadata = nil
		}
		out = append(out, a)
	}
	return out
}
