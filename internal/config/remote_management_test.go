package config

import (
	"testing"
)

func TestParseConfigBytesRemoteManagementDefaults(t *testing.T) {
	cfg, errParse := ParseConfigBytes([]byte("port: 8317\n"))
	if errParse != nil {
		t.Fatalf("ParseConfigBytes() error = %v", errParse)
	}

	if !cfg.RemoteManagement.DisableAutoUpdatePanel {
		t.Fatal("RemoteManagement.DisableAutoUpdatePanel = false, want true")
	}
	if cfg.RemoteManagement.PanelGitHubRepository != DefaultPanelGitHubRepository {
		t.Fatalf("RemoteManagement.PanelGitHubRepository = %q, want %q", cfg.RemoteManagement.PanelGitHubRepository, DefaultPanelGitHubRepository)
	}
}

func TestParseConfigBytesRemoteManagementAllowsAutoUpdates(t *testing.T) {
	cfg, errParse := ParseConfigBytes([]byte(`remote-management:
  disable-auto-update-panel: false
`))
	if errParse != nil {
		t.Fatalf("ParseConfigBytes() error = %v", errParse)
	}

	if cfg.RemoteManagement.DisableAutoUpdatePanel {
		t.Fatal("RemoteManagement.DisableAutoUpdatePanel = true, want false")
	}
}

func TestParseConfigBytesRemoteManagementBaseURL(t *testing.T) {
	cfg, errParse := ParseConfigBytes([]byte(`remote-management:
  allow-remote: true
  base-url: "https://proxy.example.com"
`))
	if errParse != nil {
		t.Fatalf("ParseConfigBytes() error = %v", errParse)
	}

	if cfg.RemoteManagement.BaseURL != "https://proxy.example.com" {
		t.Fatalf("RemoteManagement.BaseURL = %q, want https://proxy.example.com", cfg.RemoteManagement.BaseURL)
	}
}
