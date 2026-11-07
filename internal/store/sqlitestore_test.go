package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	xinugateauth "github.com/x-inu/XinuGate/v7/sdk/xinugate/auth"
)

func TestSqliteStoreLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_data.sqlite")
	spoolDir := filepath.Join(tempDir, "spool")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s, err := NewSqliteStore(ctx, SqliteStoreConfig{
		DBPath:   dbPath,
		SpoolDir: spoolDir,
	})
	if err != nil {
		t.Fatalf("NewSqliteStore failed: %v", err)
	}
	defer s.Close()

	// 1. Test Bootstrap with seed config
	exampleConfig := filepath.Join(tempDir, "example_config.yaml")
	seedData := []byte("port: 20128\nhost: '127.0.0.1'\n")
	if err := os.WriteFile(exampleConfig, seedData, 0o600); err != nil {
		t.Fatalf("failed to write example config: %v", err)
	}

	if err := s.Bootstrap(ctx, exampleConfig); err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	// Verify config file in spool
	cfgOnDisk, err := os.ReadFile(s.ConfigPath())
	if err != nil {
		t.Fatalf("read config from spool failed: %v", err)
	}
	if string(cfgOnDisk) != string(seedData) {
		t.Fatalf("config content mismatch: got %q, want %q", string(cfgOnDisk), string(seedData))
	}

	// 2. Test Save Auth
	auth := &xinugateauth.Auth{
		ID:       "test-account-1",
		FileName: "antigravity-account-1.json",
		Metadata: map[string]any{
			"email":    "user@example.com",
			"provider": "antigravity",
		},
	}
	path, err := s.Save(ctx, auth)
	if err != nil {
		t.Fatalf("Save auth failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected auth file on disk at %s, got: %v", path, err)
	}

	// 3. Test Get Auth
	retrieved, err := s.Get(ctx, "antigravity-account-1.json")
	if err != nil {
		t.Fatalf("Get auth failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected retrieved auth to be non-nil")
	}
	if retrieved.Metadata["email"] != "user@example.com" {
		t.Fatalf("expected email user@example.com, got %v", retrieved.Metadata["email"])
	}

	// 4. Test List Auths
	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List auths failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 auth in list, got %d", len(list))
	}

	// 5. Test Cooldown Store Save & Load
	cdStore := s.CooldownStateStore()
	if cdStore == nil {
		t.Fatal("expected CooldownStateStore to be non-nil")
	}
	retryAfter := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	testRecords := []xinugateauth.CooldownStateRecord{
		{
			AuthID:         "test-account-1",
			Model:          "gemini-3.8-flash-high",
			Status:         "rate_limited",
			NextRetryAfter: retryAfter,
		},
	}
	if err := cdStore.Save(ctx, testRecords); err != nil {
		t.Fatalf("CooldownStateStore.Save failed: %v", err)
	}

	loadedRecords, err := cdStore.Load(ctx)
	if err != nil {
		t.Fatalf("CooldownStateStore.Load failed: %v", err)
	}
	if len(loadedRecords) != 1 {
		t.Fatalf("expected 1 cooldown record, got %d", len(loadedRecords))
	}
	if loadedRecords[0].AuthID != "test-account-1" || loadedRecords[0].Model != "gemini-3.8-flash-high" {
		t.Fatalf("loaded cooldown record mismatch: %+v", loadedRecords[0])
	}

	// 6. Test Delete Auth
	if err := s.Delete(ctx, "antigravity-account-1.json"); err != nil {
		t.Fatalf("Delete auth failed: %v", err)
	}
	listAfterDelete, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List after delete failed: %v", err)
	}
	if len(listAfterDelete) != 0 {
		t.Fatalf("expected 0 auths after delete, got %d", len(listAfterDelete))
	}
}

func TestSqliteStoreBootstrapWithSeeds(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "data.sqlite")
	spoolDir := filepath.Join(tempDir, "spool")
	seedAuthDir := filepath.Join(tempDir, "existing_auths")
	if err := os.MkdirAll(seedAuthDir, 0o700); err != nil {
		t.Fatalf("mkdir seed auth dir: %v", err)
	}

	// Create 2 existing auth files
	auth1 := []byte(`{"email":"acc1@example.com","provider":"antigravity"}`)
	auth2 := []byte(`{"email":"acc2@example.com","provider":"codex"}`)
	_ = os.WriteFile(filepath.Join(seedAuthDir, "antigravity-acc1.json"), auth1, 0o600)
	_ = os.WriteFile(filepath.Join(seedAuthDir, "codex-acc2.json"), auth2, 0o600)

	// Create existing config
	seedConfigFile := filepath.Join(tempDir, "existing_config.yaml")
	seedConfigData := []byte("port: 20128\nremote-management:\n  allow-remote: true\n")
	_ = os.WriteFile(seedConfigFile, seedConfigData, 0o600)

	ctx := context.Background()
	s, err := NewSqliteStore(ctx, SqliteStoreConfig{
		DBPath:   dbPath,
		SpoolDir: spoolDir,
	})
	if err != nil {
		t.Fatalf("NewSqliteStore failed: %v", err)
	}
	defer s.Close()

	if err := s.BootstrapWithSeeds(ctx, seedConfigFile, seedAuthDir); err != nil {
		t.Fatalf("BootstrapWithSeeds failed: %v", err)
	}

	// Verify both auth files exist in sqlite
	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("s.List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 auth files in sqlite, got %d", len(list))
	}

	// Verify config exists in spool and matches
	spoolCfg, err := os.ReadFile(s.ConfigPath())
	if err != nil {
		t.Fatalf("read spool config: %v", err)
	}
	if string(spoolCfg) != string(seedConfigData) {
		t.Fatalf("expected spool config to match seed config, got: %s", string(spoolCfg))
	}
}
