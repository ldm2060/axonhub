package oauth

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type OAuthCredentials struct {
	ClientID     string            `json:"client_id,omitempty"`
	AccessToken  string            `json:"access_token"`
	RefreshToken string            `json:"refresh_token"`
	IDToken      string            `json:"id_token,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at"`
	TokenType    string            `json:"token_type,omitempty"`
	Scopes       []string          `json:"scopes,omitempty"`
	KimiCode     *KimiCodeMetadata `json:"kimi_code,omitempty"`
}

// KimiCodeMetadata is persisted with OAuth credentials so Kimi's dynamic
// protocol catalog remains available when a channel cache is rebuilt.
type KimiCodeMetadata struct {
	Models []KimiCodeModel `json:"models"`
}

type KimiCodeModel struct {
	ID                   string                `json:"id"`
	ContextLength        int64                 `json:"context_length"`
	Protocol             string                `json:"protocol,omitempty"`
	SupportsReasoning    bool                  `json:"supports_reasoning,omitempty"`
	SupportsImageIn      bool                  `json:"supports_image_in,omitempty"`
	SupportsVideoIn      bool                  `json:"supports_video_in,omitempty"`
	SupportsToolUse      bool                  `json:"supports_tool_use,omitempty"`
	SupportsThinkingType string                `json:"supports_thinking_type,omitempty"`
	ThinkEfforts         *KimiCodeThinkEfforts `json:"think_efforts,omitempty"`
}

type KimiCodeThinkEfforts struct {
	Support       bool     `json:"support,omitempty"`
	ValidEfforts  []string `json:"valid_efforts,omitempty"`
	DefaultEffort string   `json:"default_effort,omitempty"`
}

type TokenResponse struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope,omitempty"`
}

// ExpiresAt calculates the expiration time based on ExpiresIn.
func (t *TokenResponse) ExpiresAt() time.Time {
	if t.ExpiresIn > 0 {
		return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	}

	return time.Time{}
}

type TokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func ParseCredentialsJSON(raw string) (*OAuthCredentials, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("empty credentials")
	}

	var creds OAuthCredentials
	if err := json.Unmarshal([]byte(trimmed), &creds); err != nil {
		return nil, err
	}

	if creds.AccessToken == "" {
		return nil, errors.New("access_token is empty")
	}

	// If refresh_token exists but expires_at is missing, assume 1 hour.
	if creds.RefreshToken != "" && creds.ExpiresAt.IsZero() {
		creds.ExpiresAt = time.Now().Add(1 * time.Hour)
	}

	return &creds, nil
}

func (c *OAuthCredentials) IsExpired(now time.Time) bool {
	if c == nil {
		return true
	}

	if c.ExpiresAt.IsZero() {
		return true
	}

	// Consider token expired 3 minutes earlier.
	return now.Add(3 * time.Minute).After(c.ExpiresAt)
}

func (c *OAuthCredentials) ToJSON() (string, error) {
	b, err := json.Marshal(c) //nolint:gosec
	if err != nil {
		return "", err
	}

	return string(b), nil
}
