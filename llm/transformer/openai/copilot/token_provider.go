package copilot

import (
	"context"
	"errors"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

// CopilotTokenProvider manages OAuth2 credentials for the Copilot API.
// GitHub's Copilot OAuth tokens from the device flow are long-lived and
// do not include a refresh_token. The access_token is used directly as
// the Bearer token — no exchange or refresh step is needed.
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

// GetToken returns the OAuth access_token directly.
// GitHub's Copilot device flow tokens are long-lived and do not require
// refresh — they are used as-is for Bearer authentication, matching
// opencode's implementation.
func (p *CopilotTokenProvider) GetToken(_ context.Context) (string, error) {
	creds := p.deviceFlowProvider.GetCredentials()
	if creds == nil {
		return "", errors.New("credentials is nil")
	}

	if creds.AccessToken == "" {
		return "", errors.New("access_token is empty")
	}

	return creds.AccessToken, nil
}

// UpdateCredentials updates the stored OAuth credentials.
func (p *CopilotTokenProvider) UpdateCredentials(creds *oauth.OAuthCredentials) {
	if p.deviceFlowProvider != nil {
		p.deviceFlowProvider.UpdateCredentials(creds)
	}
}

// GetCredentials returns a copy of the current OAuth credentials.
func (p *CopilotTokenProvider) GetCredentials() *oauth.OAuthCredentials {
	return p.deviceFlowProvider.GetCredentials()
}

// StartAutoRefresh is a no-op for Copilot.
// GitHub's Copilot device flow tokens are long-lived and don't have a
// refresh_token, so background refresh is not applicable.
func (p *CopilotTokenProvider) StartAutoRefresh(_ context.Context, _ oauth.AutoRefreshOptions) {}

// StopAutoRefresh is a no-op for Copilot.
func (p *CopilotTokenProvider) StopAutoRefresh() {}
