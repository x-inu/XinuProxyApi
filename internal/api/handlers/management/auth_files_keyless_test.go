package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/watcher/synthesizer"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestListAuthFiles_NeverIncludesKeylessOrConfigOpenAICompat(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")

	cfg := &config.Config{
		AuthDir: t.TempDir(),
		OpenAICompatibility: []config.OpenAICompatibility{
			{
				Name:    "opencodeFree",
				BaseURL: "https://opencode.ai/zen/v1",
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{
					{APIKey: "", ProxyURL: "http://proxy1:8080"},
					{APIKey: "", ProxyURL: "http://proxy2:8080"},
					{APIKey: ""},
				},
			},
		},
	}

	synth := synthesizer.NewConfigSynthesizer()
	sctx := &synthesizer.SynthesisContext{
		Config:      cfg,
		AuthDir:     cfg.AuthDir,
		Now:         time.Now(),
		IDGenerator: synthesizer.NewStableIDGenerator(),
	}

	auths, err := synth.Synthesize(sctx)
	if err != nil {
		t.Fatalf("synthesize failed: %v", err)
	}
	if len(auths) != 3 {
		t.Fatalf("expected 3 synthesized auths, got %d", len(auths))
	}

	manager := coreauth.NewManager(nil, nil, nil)
	for _, a := range auths {
		registerAuthForLookupTest(t, manager, a)
	}

	h := NewHandlerWithoutConfigFilePath(cfg, manager)

	// Test non-paginated
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Files) != 0 {
		t.Fatalf("expected 0 files in auth-files, but got %d: %+v", len(resp.Files), resp.Files)
	}

	// Test paginated
	recPaging := httptest.NewRecorder()
	ctxPaging, _ := gin.CreateTestContext(recPaging)
	ctxPaging.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files?page=1&page_size=10", nil)
	h.ListAuthFiles(ctxPaging)

	if recPaging.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recPaging.Code)
	}

	var respPaging struct {
		Files []map[string]any `json:"files"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(recPaging.Body.Bytes(), &respPaging); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(respPaging.Files) != 0 || respPaging.Total != 0 {
		t.Fatalf("expected 0 files in paginated auth-files, but got total=%d: %+v", respPaging.Total, respPaging.Files)
	}
}
