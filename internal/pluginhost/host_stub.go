package pluginhost

// Stub host: dynamic plugin engine removed. All capabilities report unhandled so the
// core gateway continues with built-in executors, schedulers, and interceptors only.

import (
	"context"
	"flag"
	"net/http"

	"github.com/x-inu/XinuGate/v7/internal/config"
	"github.com/x-inu/XinuGate/v7/internal/interfaces"
	"github.com/x-inu/XinuGate/v7/internal/registry"
	"github.com/x-inu/XinuGate/v7/sdk/api/handlers"
	"github.com/x-inu/XinuGate/v7/sdk/pluginapi"
	sdktranslator "github.com/x-inu/XinuGate/v7/sdk/translator"
	coreauth "github.com/x-inu/XinuGate/v7/sdk/xinugate/auth"
	coreexecutor "github.com/x-inu/XinuGate/v7/sdk/xinugate/executor"
)

type modelExecutor interface {
	ExecuteModel(context.Context, handlers.ModelExecutionRequest) (handlers.ModelExecutionResponse, *interfaces.ErrorMessage)
	ExecuteModelStream(context.Context, handlers.ModelExecutionRequest) (handlers.ModelExecutionStream, *interfaces.ErrorMessage)
}

type AuthModelResult struct {
	Provider string
	Models   []*registry.ModelInfo
	Auth     *coreauth.Auth
	Handled  bool
	Err      error
}

type RegisteredPluginInfo struct {
	ID            string
	Priority      int
	Metadata      pluginapi.Metadata
	SupportsOAuth bool
	OAuthProvider string
	SupportsQuota bool
	QuotaProvider string
	Menus         []RegisteredPluginMenu
}

type RegisteredPluginMenu struct {
	Path        string
	Menu        string
	Description string
}

type RegisteredQuotaProviderInfo struct {
	PluginID           string   `json:"plugin_id"`
	Provider           string   `json:"provider"`
	DisplayName        string   `json:"display_name,omitempty"`
	SupportedProviders []string `json:"supported_providers,omitempty"`
	SupportsReset      bool     `json:"supports_reset"`
}

type Snapshot struct{}

type Host struct{}

func New() *Host { return &Host{} }

// NewForTest kept for compatibility; test loader ignored.
func NewForTest(loader any) *Host { return &Host{} }

func SupportPluginHeaderValue() string { return "0" }

func (h *Host) SetModelExecutor(executor modelExecutor)                 {}
func (h *Host) SetAuthManager(manager *coreauth.Manager)                {}
func (h *Host) ApplyConfig(ctx context.Context, cfg *config.Config)     {}
func (h *Host) Snapshot() *Snapshot                                     { return &Snapshot{} }
func (h *Host) PluginLoaded(id string) bool                             { return false }
func (h *Host) PluginBusy(id string) bool                               { return false }
func (h *Host) PluginRegistered(id string) bool                         { return false }
func (h *Host) UnloadPlugin(id string) bool                             { return false }
func (h *Host) UnloadPluginContext(ctx context.Context, id string) bool { return false }
func (h *Host) ShutdownAll()                                            {}
func (h *Host) ShutdownAllContext(ctx context.Context)                  {}

func (h *Host) AuthProviderIdentifiers() []string                                 { return nil }
func (h *Host) BuiltinProviders() []string                                        { return nil }
func (h *Host) HasAuthProvider(provider string) bool                              { return false }
func (h *Host) HasBuiltinProvider(provider string) bool                           { return false }
func (h *Host) HasExecutorCandidateProvider(provider string) bool                 { return false }
func (h *Host) HasModelRouters() bool                                             { return false }
func (h *Host) HasModelRoutersExcept(skipPluginID string) bool                    { return false }
func (h *Host) HasQuotaProviderContext(ctx context.Context, provider string) bool { return false }
func (h *Host) HasQuotaProviderForPlugin(pluginID string) bool                    { return false }
func (h *Host) HasQuotaProvider(provider string) bool                             { return false }
func (h *Host) HasRequestInterceptors() bool                                      { return false }
func (h *Host) HasScheduler() bool                                                { return false }
func (h *Host) HasStreamInterceptors() bool                                       { return false }
func (h *Host) HasTriggeredCommandLineFlags() bool                                { return false }
func (h *Host) HasWebSocketResponseObservers() bool                               { return false }
func (h *Host) QuotaProviderIdentifiers() []string                                { return nil }
func (h *Host) SchedulerWantsAcrossPriorities() bool                              { return false }
func (h *Host) StreamChunkPayloadIncludesHistory() bool                           { return false }
func (h *Host) StreamChunkPayloadIncludesRequestBody() bool                       { return false }

func (h *Host) ModelsForAuth(ctx context.Context, auth *coreauth.Auth) AuthModelResult {
	return AuthModelResult{}
}
func (h *Host) ModelsForProvider(provider string) []*registry.ModelInfo { return nil }
func (h *Host) AuthDataToCoreAuth(data pluginapi.AuthData, path, fileName string) *coreauth.Auth {
	return nil
}
func (h *Host) ParseAuth(ctx context.Context, req pluginapi.AuthParseRequest) (*coreauth.Auth, bool, error) {
	return nil, false, nil
}
func (h *Host) ParseAuths(ctx context.Context, req pluginapi.AuthParseRequest) ([]*coreauth.Auth, bool, error) {
	return nil, false, nil
}
func (h *Host) RefreshAuth(ctx context.Context, auth *coreauth.Auth) (*coreauth.Auth, bool, error) {
	return nil, false, nil
}
func (h *Host) StartLogin(ctx context.Context, provider string, baseURL string, metadata ...map[string]any) (pluginapi.AuthLoginStartResponse, bool, error) {
	return pluginapi.AuthLoginStartResponse{}, false, nil
}
func (h *Host) PollLogin(ctx context.Context, provider, state string, metadata ...map[string]any) (pluginapi.AuthLoginPollResponse, bool, error) {
	return pluginapi.AuthLoginPollResponse{}, false, nil
}
func (h *Host) PickAuth(ctx context.Context, req pluginapi.SchedulerPickRequest) (pluginapi.SchedulerPickResponse, bool, error) {
	return pluginapi.SchedulerPickResponse{}, false, nil
}
func (h *Host) InterceptRequestBeforeAuth(ctx context.Context, req pluginapi.RequestInterceptRequest) pluginapi.RequestInterceptResponse {
	return pluginapi.RequestInterceptResponse{}
}
func (h *Host) InterceptRequestBeforeAuthExcept(ctx context.Context, req pluginapi.RequestInterceptRequest, skipPluginID string) pluginapi.RequestInterceptResponse {
	return pluginapi.RequestInterceptResponse{}
}
func (h *Host) InterceptRequestAfterAuth(ctx context.Context, req pluginapi.RequestInterceptRequest) pluginapi.RequestInterceptResponse {
	return pluginapi.RequestInterceptResponse{}
}
func (h *Host) InterceptRequestAfterAuthExcept(ctx context.Context, req pluginapi.RequestInterceptRequest, skipPluginID string) pluginapi.RequestInterceptResponse {
	return pluginapi.RequestInterceptResponse{}
}
func (h *Host) InterceptResponse(ctx context.Context, req pluginapi.ResponseInterceptRequest) pluginapi.ResponseInterceptResponse {
	return pluginapi.ResponseInterceptResponse{}
}
func (h *Host) InterceptResponseExcept(ctx context.Context, req pluginapi.ResponseInterceptRequest, skipPluginID string) pluginapi.ResponseInterceptResponse {
	return pluginapi.ResponseInterceptResponse{}
}
func (h *Host) InterceptStreamChunk(ctx context.Context, req pluginapi.StreamChunkInterceptRequest) pluginapi.StreamChunkInterceptResponse {
	return pluginapi.StreamChunkInterceptResponse{}
}
func (h *Host) InterceptStreamChunkExcept(ctx context.Context, req pluginapi.StreamChunkInterceptRequest, skipPluginID string) pluginapi.StreamChunkInterceptResponse {
	return pluginapi.StreamChunkInterceptResponse{}
}
func (h *Host) CompleteRequest(ctx context.Context, completion pluginapi.RequestCompletion) {}
func (h *Host) CompleteRequestExcept(ctx context.Context, completion pluginapi.RequestCompletion, skipPluginID string) {
}
func (h *Host) ObserveWebSocketResponseEvent(ctx context.Context, event pluginapi.WebSocketResponseEvent) {
}
func (h *Host) ObserveWebSocketResponseEventExcept(ctx context.Context, event pluginapi.WebSocketResponseEvent, skipPluginID string) {
}
func (h *Host) NormalizeRequest(ctx context.Context, from, to sdktranslator.Format, model string, body []byte, stream bool) []byte {
	return body
}
func (h *Host) NormalizeResponseBefore(ctx context.Context, from, to sdktranslator.Format, model string, originalRequestRawJSON, requestRawJSON, body []byte, stream bool) []byte {
	return body
}
func (h *Host) NormalizeResponseAfter(ctx context.Context, from, to sdktranslator.Format, model string, originalRequestRawJSON, requestRawJSON, body []byte, stream bool) []byte {
	return body
}
func (h *Host) TranslateRequest(ctx context.Context, from, to sdktranslator.Format, model string, body []byte, stream bool) ([]byte, bool) {
	return body, false
}
func (h *Host) TranslateResponse(ctx context.Context, from, to sdktranslator.Format, model string, originalRequestRawJSON, requestRawJSON, body []byte, stream bool) ([]byte, bool) {
	return body, false
}
func (h *Host) RouteModel(ctx context.Context, req pluginapi.ModelRouteRequest) (pluginapi.ModelRouteResponse, bool) {
	return pluginapi.ModelRouteResponse{}, false
}
func (h *Host) RouteModelExcept(ctx context.Context, req pluginapi.ModelRouteRequest, skipPluginID string) (pluginapi.ModelRouteResponse, bool) {
	return pluginapi.ModelRouteResponse{}, false
}
func (h *Host) DescribeQuota(ctx context.Context, pluginID string) (pluginapi.QuotaDescribeResponse, bool, error) {
	return pluginapi.QuotaDescribeResponse{}, false, nil
}
func (h *Host) FetchQuotaByPlugin(ctx context.Context, pluginID string, req pluginapi.QuotaFetchRequest) (pluginapi.QuotaFetchResponse, bool, error) {
	return pluginapi.QuotaFetchResponse{}, false, nil
}
func (h *Host) FetchQuota(ctx context.Context, req pluginapi.QuotaFetchRequest) (pluginapi.QuotaFetchResponse, bool, error) {
	return pluginapi.QuotaFetchResponse{}, false, nil
}
func (h *Host) ResetQuotaByPlugin(ctx context.Context, pluginID string, req pluginapi.QuotaResetRequest) (pluginapi.QuotaResetResponse, bool, error) {
	return pluginapi.QuotaResetResponse{}, false, nil
}
func (h *Host) ResetQuota(ctx context.Context, req pluginapi.QuotaResetRequest) (pluginapi.QuotaResetResponse, bool, error) {
	return pluginapi.QuotaResetResponse{}, false, nil
}
func (h *Host) QuotaProviders(ctx context.Context) []RegisteredQuotaProviderInfo   { return nil }
func (h *Host) QuotaSupportedProvidersSet(ctx context.Context) map[string]struct{} { return nil }
func (h *Host) CountPluginExecutor(ctx context.Context, pluginID string, req coreexecutor.Request, opts coreexecutor.Options) (coreexecutor.Response, error) {
	return coreexecutor.Response{}, nil
}
func (h *Host) ExecutePluginExecutor(ctx context.Context, pluginID string, req coreexecutor.Request, opts coreexecutor.Options) (coreexecutor.Response, error) {
	return coreexecutor.Response{}, nil
}
func (h *Host) ExecutePluginExecutorStream(ctx context.Context, pluginID string, req coreexecutor.Request, opts coreexecutor.Options) (*coreexecutor.StreamResult, error) {
	return nil, nil
}
func (h *Host) PluginExecutorRequestToFormat(pluginID string, req coreexecutor.Request, opts coreexecutor.Options) sdktranslator.Format {
	return ""
}
func (h *Host) OwnsExecutor(executor coreauth.ProviderExecutor) bool                           { return false }
func (h *Host) RegisterExecutors(manager executorManager, modelRegistry modelProviderRegistry) {}
func (h *Host) RegisterFrontendAuthProviders()                                                 {}
func (h *Host) RegisterManagementRoutes(ctx context.Context, reserved map[string]struct{})     {}
func (h *Host) RegisterModels(ctx context.Context, modelRegistry modelRegistry)                {}
func (h *Host) RegisterUsagePlugins()                                                          {}
func (h *Host) RegisterPluginForTest(id string, plugin pluginapi.Plugin)                       {}
func (h *Host) RegisterCommandLineFlags(ctx context.Context, flagSet *flag.FlagSet)            {}
func (h *Host) ExecuteCommandLine(ctx context.Context, program string, args []string, configPath string, flagSet *flag.FlagSet) (int, bool) {
	return 0, false
}
func (h *Host) RegisteredPlugins() []RegisteredPluginInfo                       { return nil }
func (h *Host) ServeManagementHTTP(w http.ResponseWriter, r *http.Request) bool { return false }
func (h *Host) ServeResourceHTTP(w http.ResponseWriter, r *http.Request) bool   { return false }

// Compatibility helpers for executor registration and refresh compat wrappers.

type executorManager interface {
	Executor(provider string) (coreauth.ProviderExecutor, bool)
	RegisterExecutor(coreauth.ProviderExecutor)
	UnregisterExecutor(provider string)
}

type modelRegistry interface {
	RegisterClient(clientID, clientProvider string, models []*registry.ModelInfo)
	UnregisterClient(clientID string)
}

type modelProviderRegistry interface {
	modelRegistry
	GetModelProviders(modelID string) []string
}

func NewPluginRefreshCompatExecutor(inner coreauth.ProviderExecutor, host *Host, cfg *config.Config) coreauth.ProviderExecutor {
	return inner
}

func IsPluginRefreshCompatExecutor(executor coreauth.ProviderExecutor) bool { return false }

func UnwrapPluginRefreshCompatExecutor(executor coreauth.ProviderExecutor) (coreauth.ProviderExecutor, bool) {
	return executor, false
}
