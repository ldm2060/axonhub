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

	// RedirectURI is the CLI manual-mode callback the authorize page redirects
	// to. Nothing listens on that port from the browser's perspective; the
	// user copies the failed-navigation URL (which carries code + state) back
	// into the dialog, exactly like the Claude Code OAuth flow.
	RedirectURI = "http://127.0.0.1:9999/callback"
)
