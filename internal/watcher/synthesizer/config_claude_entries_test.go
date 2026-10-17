package synthesizer

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func claudeAuthCount(auths []*coreauth.Auth) int {
	count := 0
	for _, auth := range auths {
		if auth != nil && auth.Provider == "claude" {
			count++
		}
	}
	return count
}

func TestConfigSynthesizer_ClaudeAPIKeyEntries(t *testing.T) {
	weight := 3
	cfg := &config.Config{
		ClaudeKey: []config.ClaudeKey{
			{
				// Multi-key entry: three keys share one base URL and one
				// models/excluded configuration.
				BaseURL: "https://gateway.example.com",
				APIKeyEntries: []config.ClaudeAPIKeyEntry{
					{APIKey: " key-a "},
					{APIKey: "key-b", Weight: &weight, ProxyURL: "http://proxy-b:8080"},
					{APIKey: "", ProxyURL: "http://ignored"}, // dropped: no key
				},
				Models: []config.ClaudeModel{{Name: "claude-sonnet-4-5"}},
			},
			{
				// Legacy single-key entry keeps exactly one auth.
				APIKey:  "legacy-key",
				BaseURL: "https://legacy.example.com",
			},
			{
				// Multi-key entry with no usable key synthesizes nothing.
				BaseURL: "https://empty.example.com",
				APIKeyEntries: []config.ClaudeAPIKeyEntry{
					{APIKey: "  "},
				},
			},
		},
	}
	ctx := &SynthesisContext{
		Config:      cfg,
		Now:         time.Now(),
		IDGenerator: NewStableIDGenerator(),
	}
	synth := NewConfigSynthesizer()
	auths, err := synth.Synthesize(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := claudeAuthCount(auths); got != 3 {
		t.Fatalf("claude auths = %d, want 3 (2 usable keys + 1 legacy)", got)
	}

	var multi []*coreauth.Auth
	for _, auth := range auths {
		if auth.Provider != "claude" {
			continue
		}
		if auth.Attributes["config_index"] == "0" {
			multi = append(multi, auth)
		}
	}
	if len(multi) != 2 {
		t.Fatalf("multi-key auths = %d, want 2", len(multi))
	}

	slots := map[string]bool{}
	seenIDs := map[string]bool{}
	for _, auth := range auths {
		if seenIDs[auth.ID] {
			t.Fatalf("duplicate auth id %q", auth.ID)
		}
		seenIDs[auth.ID] = true
		if auth.Provider != "claude" {
			continue
		}
		if auth.Attributes["config_index"] != "0" {
			continue
		}
		slot := auth.Attributes["key_slot"]
		slots[slot] = true

		switch slot {
		case "0":
			if auth.Attributes["api_key"] != "key-a" {
				t.Fatalf("slot 0 api_key = %q", auth.Attributes["api_key"])
			}
			if auth.ProxyURL != "" {
				t.Fatalf("slot 0 proxy = %q, want entry-level empty", auth.ProxyURL)
			}
		case "1":
			if auth.Attributes["api_key"] != "key-b" {
				t.Fatalf("slot 1 api_key = %q", auth.Attributes["api_key"])
			}
			if auth.ProxyURL != "http://proxy-b:8080" {
				t.Fatalf("slot 1 proxy = %q, want per-key proxy", auth.ProxyURL)
			}
			if auth.Attributes["weight"] != "3" {
				t.Fatalf("slot 1 weight attr = %q, want \"3\"", auth.Attributes["weight"])
			}
		case "2":
			t.Fatalf("empty key must not synthesize an auth")
		}
		if auth.Attributes["base_url"] != "https://gateway.example.com" {
			t.Fatalf("slot %q base_url = %q", slot, auth.Attributes["base_url"])
		}
	}
	if slots["0"] == false || slots["1"] == false {
		t.Fatalf("missing key slots, got %v", slots)
	}

	for _, auth := range auths {
		if auth.Provider == "claude" && auth.Attributes["config_index"] == "1" {
			if auth.Attributes["api_key"] != "legacy-key" {
				t.Fatalf("legacy api_key = %q", auth.Attributes["api_key"])
			}
			if _, hasSlot := auth.Attributes["key_slot"]; hasSlot {
				t.Fatalf("legacy auth must not carry key_slot")
			}
		}
	}
}

func TestConfigSynthesizer_ClaudeAPIKeyEntries_LegacyIDsStable(t *testing.T) {
	// A single-key entry must synthesize the exact same ID before and after
	// the multi-key feature so existing usage history keeps its identity.
	cfg := &config.Config{
		ClaudeKey: []config.ClaudeKey{
			{APIKey: "key-a", BaseURL: "https://a.example.com"},
		},
	}
	ctx := &SynthesisContext{
		Config:      cfg,
		Now:         time.Now(),
		IDGenerator: NewStableIDGenerator(),
	}
	synth := NewConfigSynthesizer()
	auths, err := synth.Synthesize(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(auths) == 0 {
		t.Fatalf("expected one auth")
	}
	if auths[0].Attributes["key_slot"] != "" {
		t.Fatalf("single-key entry must not carry key_slot")
	}
}
