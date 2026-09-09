// Package zcode implements the OAuth flow of the ZCode desktop client
// (zcode.z.ai, Z.AI's GLM coding harness), reverse engineered from the
// official client — see vibe-coding-labs/zcode-reverse-engineer.
//
// The flow differs from a standard OAuth2 provider in two ways: the token
// endpoint speaks a z.ai business envelope ({code, msg, data:{zai:{...}}})
// instead of a plain token response, and the resulting OAuth access token is
// only an intermediate credential — inference calls authenticate with the
// business JWT exchanged from api.z.ai/api/auth/z/login.
package zcode

import (
	"net/url"
	"strings"
)

const (
	// AuthorizeURL is the z.ai authorization page the user logs in on.
	AuthorizeURL = "https://chat.z.ai/api/oauth/authorize"
	// TokenURL exchanges an authorization code (or refresh token) for the
	// z.ai OAuth access token.
	//nolint:gosec // false alert.
	TokenURL = "https://zcode.z.ai/api/v1/oauth/token"
	// BusinessLoginURL exchanges the OAuth access token for the business JWT
	// used as the inference credential.
	BusinessLoginURL = "https://api.z.ai/api/auth/z/login"

	// ClientID is the ZCode desktop client's public OAuth app id. ZCode has no
	// client secret and no PKCE.
	ClientID = "client_P8X5CMWmlaRO9gyO-KSqtg"

	// Provider is the provider key inside the token endpoint's data envelope.
	Provider = "zai"

	// BigModelAuthorizeURL is the bigmodel.cn login page for the ZCode client's
	// BigModel provider. Unlike the Z.AI flow it uses custom query params
	// (appId/redirect/state, no client_id/response_type).
	BigModelAuthorizeURL = "https://bigmodel.cn/login"
	// BigModelAppID is the ZCode client's BigModel app id.
	BigModelAppID = "zcode"
	// BigModelProvider is the provider key for the BigModel token exchange; the
	// response nests the provider token under data.bigmodel and returns the
	// zcode JWT directly as data.token (no separate business-login step).
	BigModelProvider = "bigmodel"

	// RedirectURI is the z.ai-registered callback the authorize page redirects
	// to after consent, taken from the current ZCode desktop client (verified
	// against the 3.10.2 app.asar: both providers share this single value and
	// register the zcode:// scheme with the OS via setAsDefaultProtocolClient).
	// The loopback variants from earlier client versions
	// (http://127.0.0.1:9999/callback, http://127.0.0.1:{port}/oauth/callback/zai)
	// and the v3.0.1 zcode://zai-auth/callback host are all de-registered —
	// chat.z.ai rejects them with "Redirect URI not registered for this client".
	// In the manual flow the browser tries to hand the zcode:// URL to the OS
	// and the user copies the full link (carrying code + state) back into the
	// dialog — Firefox shows it in the error page's address bar; Chrome/Edge
	// log it in the DevTools console after dismissing the "Open ZCode" prompt.
	RedirectURI = "zcode://oauth/callback"

	// BigModelDesktopRedirectPath is the path of the desktop OAuth login bridge
	// on the zcode endpoint origin.
	BigModelDesktopRedirectPath = "/app/oauth/login"
)

// Runtime endpoint origins, mirrored from the ZCode desktop client
// (3.10.2 asar, resolveRuntimeZCodeEndpointOrigin): production and test
// zcode origins, selected by ZCODE_ENV.
const (
	EndpointOriginProduction = "https://zcode.z.ai"
	EndpointOriginTest       = "https://zcode.chatglm.site"
)

// Env lookup keys used by the desktop client to resolve the runtime endpoint
// origin — see EndpointOrigin.
const (
	EnvKeyEnv             = "ZCODE_ENV"
	EnvKeyBaseURL         = "ZCODE_BASE_URL"
	EnvKeyEndpointOrigin  = "ZCODE_ENDPOINT_ORIGIN"
	EnvKeyProductionURL   = "ZCODE_PRODUCTION_BASE_URL"
	EnvKeyTestURL         = "ZCODE_TEST_BASE_URL"
	EnvValueEnvTest       = "test"
	EnvValueEnvProduction = "production"
)

// EndpointOrigin resolves the runtime zcode endpoint origin the same way the
// desktop client does, via the env chain ZCODE_BASE_URL / ZCODE_ENDPOINT_ORIGIN
// / per-environment URL, falling back to the production origin. getenv is
// injected so callers can pass os.Getenv.
func EndpointOrigin(getenv func(string) string) string {
	if v := getenv(EnvKeyBaseURL); v != "" {
		return v
	}
	if v := getenv(EnvKeyEndpointOrigin); v != "" {
		return v
	}
	if getenv(EnvKeyEnv) == EnvValueEnvTest {
		if v := getenv(EnvKeyTestURL); v != "" {
			return v
		}
		return EndpointOriginTest
	}
	if v := getenv(EnvKeyProductionURL); v != "" {
		return v
	}
	return EndpointOriginProduction
}

// DesktopRedirectURI builds the redirect_uri the desktop client actually sends
// at runtime for both providers (3.10.2+ asar, buildDesktopOAuthRedirectUriFromEnv):
// the zcode origin's desktop OAuth login bridge carrying the bare callback.
// It replaces the static zcode://oauth/callback in BOTH the authorize URL and
// the token exchange — the token endpoint validates redirect_uri against the
// authorize-time value, and a bare zcode://oauth/callback is rejected with
// code 2007.
func DesktopRedirectURI(endpointOrigin string) string {
	u, err := url.Parse(endpointOrigin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		u = &url.URL{Scheme: "https", Host: strings.TrimPrefix(EndpointOriginProduction, "https://")}
	}
	u.Path = BigModelDesktopRedirectPath
	q := u.Query()
	q.Set("redirect", RedirectURI)
	q.Set("app_version", AppVersion)
	u.RawQuery = q.Encode()
	return u.String()
}

// DefaultModels returns the z.ai coding-plan model catalog. It is the fallback
// used when the live /models listing (which authenticates with the business
// JWT) cannot be fetched — e.g. the credential is not ready yet. Matches the
// catalog returned by https://api.z.ai/api/anthropic/v1/models.
func DefaultModels() []string {
	return []string{
		"glm-4.5",
		"glm-4.5-air",
		"glm-4.6",
		"glm-4.7",
		"glm-5",
		"glm-5-turbo",
		"glm-5.1",
		"glm-5.2",
		"glm-5.3",
		"glm-5.3-flash",
	}
}
