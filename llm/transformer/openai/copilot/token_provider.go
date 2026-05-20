package copilot

import (
	"context"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

// CopilotTokenProvider manages OAuth2 credentials for the Copilot API.
// It wraps oauth.DeviceFlowProvider internally to handle the device flow lifecycle.
// Uses OAuth access_token directly as Bearer token — no exchange step needed.
type CopilotTokenProvider struct {
	deviceFlowProvider *oauth.DeviceFlowProvider
}

// TokenProviderParams contains the parameters for creating a new CopilotTokenProvider.
type TokenProviderParams struct {
	Credentials *oauth.OAuthCredentials
	HTTPClient  *httpclient.HttpClient
	OnRefreshed func(ctx context.Context, refreshed *oauth.OAuthCredentials) error
}

// NewTokenProvider creates a new CopilotTokenProvider instance.
// It wraps a DeviceFlowProvider to handle the device flow lifecycle.
// The OAuth access_token is used directly for Copilot API authentication.
func NewTokenProvider(params TokenProviderParams) (*CopilotTokenProvider, error) {
	config := oauth.DeviceFlowConfig{ //nolint:gosec
		DeviceAuthURL: "https://github.com/login/device/code",
		TokenURL:      "https://github.com/login/oauth/access_token",
		ClientID:      "Iv1.b507a08c87ecfe98",
		Scopes:        []string{"read:user"},
		UserAgent:     "",
	}

	deviceFlowProvider := oauth.NewDeviceFlowProvider(oauth.DeviceFlowProviderParams{
		Config:      config,
		HTTPClient:  params.HTTPClient,
		Credentials: params.Credentials,
		OnRefreshed: params.OnRefreshed,
	})

	return &CopilotTokenProvider{
		deviceFlowProvider: deviceFlowProvider,
	}, nil
}

// GetToken returns a valid OAuth access_token for the Copilot API.
// Refreshes the token if expired using the refresh_token.
func (p *CopilotTokenProvider) GetToken(ctx context.Context) (string, error) {
	return p.deviceFlowProvider.GetToken(ctx)
}

// UpdateCredentials updates the stored OAuth credentials.
// This is called when new credentials are obtained (e.g., after device flow completes).
// Delegates to the underlying DeviceFlowProvider.
func (p *CopilotTokenProvider) UpdateCredentials(creds *oauth.OAuthCredentials) {
	if p.deviceFlowProvider != nil {
		p.deviceFlowProvider.UpdateCredentials(creds)
	}
}

// GetCredentials returns a copy of the current OAuth credentials.
// Returns nil if no credentials are stored.
// Delegates to the underlying DeviceFlowProvider.
func (p *CopilotTokenProvider) GetCredentials() *oauth.OAuthCredentials {
	return p.deviceFlowProvider.GetCredentials()
}

// StartAutoRefresh starts automatic background token refresh.
// The token will be refreshed before it expires based on the provided options.
func (p *CopilotTokenProvider) StartAutoRefresh(ctx context.Context, opts oauth.AutoRefreshOptions) {
	if p.deviceFlowProvider != nil {
		p.deviceFlowProvider.StartAutoRefresh(ctx, opts)
	}
}

// StopAutoRefresh stops automatic token refresh.
func (p *CopilotTokenProvider) StopAutoRefresh() {
	if p.deviceFlowProvider != nil {
		p.deviceFlowProvider.StopAutoRefresh()
	}
}
