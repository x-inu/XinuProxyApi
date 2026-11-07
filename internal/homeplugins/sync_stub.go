package homeplugins

import (
	"context"
	"runtime"
	"time"

	"github.com/x-inu/XinuGate/v7/internal/config"
)

type Platform struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

type PluginInstallStatus struct {
	ID            string `json:"id"`
	Version       string `json:"version,omitempty"`
	ReleaseTag    string `json:"release_tag,omitempty"`
	Repository    string `json:"repository,omitempty"`
	InstallType   string `json:"install_type,omitempty"`
	InstallStatus string `json:"install_status"`
	LoadStatus    string `json:"load_status,omitempty"`
	Path          string `json:"path,omitempty"`
	Skipped       bool   `json:"skipped,omitempty"`
	Overwritten   bool   `json:"overwritten,omitempty"`
	Error         string `json:"error,omitempty"`
}

type SyncReport struct {
	SchemaVersion int                   `json:"schema_version"`
	TaskID        uint                  `json:"task_id,omitempty"`
	Task          string                `json:"task"`
	NodeID        string                `json:"node_id,omitempty"`
	Status        string                `json:"status"`
	Phase         string                `json:"phase"`
	OK            bool                  `json:"ok"`
	StartedAt     time.Time             `json:"started_at"`
	FinishedAt    time.Time             `json:"finished_at,omitempty"`
	UpdatedAt     time.Time             `json:"updated_at"`
	Platform      Platform              `json:"platform"`
	Plugins       []PluginInstallStatus `json:"plugins"`
	Error         string                `json:"error,omitempty"`
}

func CurrentPlatform() Platform {
	return Platform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
}

func CompletedSyncReport(platform Platform, err error) SyncReport {
	return SyncReport{OK: err == nil, Platform: platform}
}

type PluginSyncRequest struct {
	SchemaVersion     int               `json:"schema_version"`
	GOOS              string            `json:"goos"`
	GOARCH            string            `json:"goarch"`
	InstalledVersions map[string]string `json:"installed_versions,omitempty"`
}

func (r *PluginSyncRequest) Clear() {}

type PluginSyncItem struct{}

func (i *PluginSyncItem) Clear() {}

type PluginSyncResponse struct {
	SchemaVersion int              `json:"schema_version"`
	ExpiresAt     time.Time        `json:"expires_at"`
	Items         []PluginSyncItem `json:"items"`
}

func (r *PluginSyncResponse) Clear() {}

func (r *PluginSyncResponse) Validate(now time.Time) error { return nil }

const PluginSyncSchemaVersion = 1

func InstalledVersions(cfg *config.Config) (map[string]string, error) {
	return nil, nil
}

func MarkLoadResults(report *SyncReport, host any) error { return nil }

func SyncWithReport(ctx context.Context, cfg *config.Config, host any) (SyncReport, error) {
	return CompletedSyncReport(CurrentPlatform(), nil), nil
}

func SyncResolvedWithReport(ctx context.Context, cfg *config.Config, items []PluginSyncItem, expiresAt time.Time, installedVersions map[string]string, host any) (SyncReport, error) {
	return CompletedSyncReport(CurrentPlatform(), nil), nil
}

func DeleteWithReport(ctx context.Context, cfg *config.Config, host any, taskID uint, pluginID string) SyncReport {
	return SyncReport{OK: true}
}
