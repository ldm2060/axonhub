package kimicode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

// DeviceAuthorization is the validated device authorization response.
type DeviceAuthorization struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// OAuthError represents a provider-defined OAuth error response.
type OAuthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *OAuthError) Error() string {
	if e == nil {
		return "kimi code oauth error"
	}
	if e.Description != "" {
		return fmt.Sprintf("kimi code oauth %s: %s", e.Code, e.Description)
	}
	return "kimi code oauth " + e.Code
}

func isLoginRequired(err error) bool {
	var oauthErr *OAuthError
	if errors.As(err, &oauthErr) && oauthErr.Code == "invalid_grant" {
		return true
	}
	var httpErr *httpclient.Error
	return errors.As(err, &httpErr) && (httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden)
}

func oauthURL(host, path string) string {
	host = strings.TrimRight(strings.TrimSpace(host), "/")
	if host == "" {
		host = DefaultOAuthHost
	}
	return host + path
}

func makeOAuthRequest(method, endpoint string, form url.Values, identity Identity) (*httpclient.Request, error) {
	headers, err := BuildIdentityHeaders(identity)
	if err != nil {
		return nil, err
	}
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	headers.Set("Accept", "application/json")
	return &httpclient.Request{Method: method, URL: endpoint, Headers: headers, Body: []byte(form.Encode())}, nil
}

// RequestDeviceAuthorization initiates the Kimi Code RFC 8628 device flow.
func RequestDeviceAuthorization(ctx context.Context, httpClient *httpclient.HttpClient, oauthHost string, identity Identity) (*DeviceAuthorization, error) {
	if httpClient == nil {
		return nil, errors.New("http client is nil")
	}
	form := url.Values{"client_id": {ClientID}}
	request, err := makeOAuthRequest(http.MethodPost, oauthURL(oauthHost, DeviceAuthorizationPath), form, identity)
	if err != nil {
		return nil, fmt.Errorf("build device authorization request: %w", err)
	}
	response, err := httpClient.Do(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("request device authorization: %w", err)
	}
	var result DeviceAuthorization
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return nil, fmt.Errorf("decode device authorization response: %w", err)
	}
	if result.DeviceCode == "" || result.UserCode == "" || result.VerificationURIComplete == "" {
		return nil, errors.New("device authorization response missing device_code, user_code, or verification_uri_complete")
	}
	if result.Interval <= 0 {
		result.Interval = 5
	}
	return &result, nil
}

// PollDeviceToken returns credentials on success or an OAuthError for an RFC
// 8628 state such as authorization_pending, slow_down, expired_token, or
// access_denied.
func PollDeviceToken(ctx context.Context, httpClient *httpclient.HttpClient, oauthHost, deviceCode string, identity Identity) (*oauth.OAuthCredentials, error) {
	if strings.TrimSpace(deviceCode) == "" {
		return nil, errors.New("device_code is empty")
	}
	form := url.Values{"client_id": {ClientID}, "device_code": {deviceCode}, "grant_type": {DeviceGrantType}}
	request, err := makeOAuthRequest(http.MethodPost, oauthURL(oauthHost, TokenPath), form, identity)
	if err != nil {
		return nil, fmt.Errorf("build device token request: %w", err)
	}
	response, err := httpClient.Do(ctx, request)
	if err != nil {
		var httpErr *httpclient.Error
		if errors.As(err, &httpErr) {
			return parseOAuthHTTPError(httpErr)
		}
		return nil, fmt.Errorf("poll device token: %w", err)
	}
	return parseToken(response.Body)
}

func parseOAuthHTTPError(httpErr *httpclient.Error) (*oauth.OAuthCredentials, error) {
	var providerError OAuthError
	if json.Unmarshal(httpErr.Body, &providerError) == nil && providerError.Code != "" {
		return nil, &providerError
	}
	return nil, httpErr
}

func parseToken(body []byte) (*oauth.OAuthCredentials, error) {
	return parseTokenWithRefreshToken(body, "")
}

func parseTokenWithRefreshToken(body []byte, fallbackRefreshToken string) (*oauth.OAuthCredentials, error) {
	var token oauth.TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if token.AccessToken == "" {
		var providerError OAuthError
		if json.Unmarshal(body, &providerError) == nil && providerError.Code != "" {
			return nil, &providerError
		}
		return nil, errors.New("token response missing access_token")
	}
	if token.RefreshToken == "" {
		token.RefreshToken = fallbackRefreshToken
	}
	if token.RefreshToken == "" {
		return nil, errors.New("token response missing refresh_token")
	}
	if token.ExpiresIn <= 0 {
		return nil, errors.New("token response missing or invalid expires_in")
	}
	return &oauth.OAuthCredentials{
		ClientID:     ClientID,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		IDToken:      token.IDToken,
		ExpiresAt:    time.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
		TokenType:    token.TokenType,
		Scopes:       strings.Fields(token.Scope),
	}, nil
}
