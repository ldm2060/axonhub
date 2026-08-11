package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/fx"
)

const (
	TurnstileActionSignIn        = "signin"
	TurnstileActionSignUpCode    = "signup_send_code"
	turnstileSiteverifyURL       = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	turnstileMaxTokenBytes       = 2048
	turnstileMaxResponseBodySize = 64 * 1024
	turnstileRequestTimeout      = 8 * time.Second
)

// TurnstileVerifier validates a single-use challenge for a server-owned action.
type TurnstileVerifier interface {
	Verify(ctx context.Context, token, expectedAction string) error
	PublicConfig(ctx context.Context) (*PublicTurnstileConfig, error)
}

// PublicTurnstileConfig contains the safe subset exposed before authentication.
type PublicTurnstileConfig struct {
	Enabled bool   `json:"enabled"`
	SiteKey string `json:"site_key"`
	Actions struct {
		SignIn     string `json:"signin"`
		SignUpCode string `json:"signup_send_code"`
	} `json:"actions"`
}

type TurnstileServiceParams struct {
	fx.In

	SystemService *SystemService
	PublicURL     string `name:"public_url"`
}

// TurnstileService performs Cloudflare Siteverify requests using an isolated HTTP client.
type TurnstileService struct {
	systemService     *SystemService
	httpClient        *http.Client
	canonicalHostname string
}

func NewTurnstileService(params TurnstileServiceParams) *TurnstileService {
	return newTurnstileService(params.SystemService, newTurnstileHTTPClient(), params.PublicURL)
}

func newTurnstileService(systemService *SystemService, httpClient *http.Client, publicURL string) *TurnstileService {
	return &TurnstileService{
		systemService:     systemService,
		httpClient:        httpClient,
		canonicalHostname: canonicalHostname(publicURL),
	}
}

func newTurnstileHTTPClient() *http.Client {
	return &http.Client{
		Timeout: turnstileRequestTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func canonicalHostname(publicURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(publicURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
}

func newPublicTurnstileConfig(settings *StoredTurnstileSettings) *PublicTurnstileConfig {
	config := &PublicTurnstileConfig{
		Enabled: settings.Enabled,
		SiteKey: settings.SiteKey,
	}
	config.Actions.SignIn = TurnstileActionSignIn
	config.Actions.SignUpCode = TurnstileActionSignUpCode

	return config
}

func (s *TurnstileService) PublicConfig(ctx context.Context) (*PublicTurnstileConfig, error) {
	settings, err := s.systemService.TurnstileSettings(ctx)
	if err != nil {
		return nil, turnstileUnavailable("settings read failed", err)
	}
	if settings.Enabled && (settings.SiteKey == "" || settings.SecretKey == "") {
		return nil, turnstileUnavailable("enabled configuration is incomplete", nil)
	}

	return newPublicTurnstileConfig(settings), nil
}

func (s *TurnstileService) Verify(ctx context.Context, token, expectedAction string) error {
	settings, err := s.systemService.TurnstileSettings(ctx)
	if err != nil {
		return turnstileUnavailable("settings read failed", err)
	}
	if !settings.Enabled {
		return nil
	}
	if settings.SiteKey == "" || settings.SecretKey == "" {
		return turnstileUnavailable("enabled configuration is incomplete", nil)
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return turnstileInvalid("challenge response is required")
	}
	if len(token) > turnstileMaxTokenBytes {
		return turnstileInvalid("challenge response is too long")
	}

	form := url.Values{}
	form.Set("secret", settings.SecretKey)
	form.Set("response", token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileSiteverifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return turnstileUnavailable("request creation failed", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return turnstileUnavailable("provider request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return turnstileUnavailable("provider returned a non-success status", nil)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, turnstileMaxResponseBodySize+1))
	if err != nil {
		return turnstileUnavailable("provider response read failed", err)
	}
	if len(body) > turnstileMaxResponseBodySize {
		return turnstileUnavailable("provider response is too large", nil)
	}

	var result siteverifyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return turnstileUnavailable("provider response is malformed", err)
	}

	return s.validateResponse(result, expectedAction)
}

type siteverifyResponse struct {
	Success    bool     `json:"success"`
	Hostname   string   `json:"hostname"`
	Action     string   `json:"action"`
	ErrorCodes []string `json:"error-codes"`
}

func (s *TurnstileService) validateResponse(result siteverifyResponse, expectedAction string) error {
	if !result.Success {
		if len(result.ErrorCodes) == 0 {
			return turnstileUnavailable("provider rejected the challenge without a reason", nil)
		}

		unavailable := false
		for _, code := range result.ErrorCodes {
			switch code {
			case "missing-input-response", "invalid-input-response", "timeout-or-duplicate":
				// A conclusively invalid, expired, or already-used user response.
			case "missing-input-secret", "invalid-input-secret", "internal-error", "bad-request":
				unavailable = true
			default:
				unavailable = true
			}
		}
		if unavailable {
			return turnstileUnavailable("provider or configuration failure", nil)
		}

		return turnstileInvalid("challenge response was rejected")
	}

	if result.Action != expectedAction {
		return turnstileInvalid("challenge action does not match")
	}
	if s.canonicalHostname != "" {
		actualHostname := strings.ToLower(strings.TrimSuffix(result.Hostname, "."))
		if actualHostname != s.canonicalHostname {
			return turnstileInvalid("challenge hostname does not match")
		}
	}

	return nil
}

func turnstileInvalid(reason string) error {
	return fmt.Errorf("%w: %s", ErrTurnstileInvalid, reason)
}

func turnstileUnavailable(reason string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: %s", ErrTurnstileUnavailable, reason)
	}

	return fmt.Errorf("%w: %s: %w", ErrTurnstileUnavailable, reason, cause)
}

// IsTurnstileError reports whether err is a public Turnstile error class.
func IsTurnstileError(err error) bool {
	return errors.Is(err, ErrTurnstileInvalid) || errors.Is(err, ErrTurnstileUnavailable)
}
