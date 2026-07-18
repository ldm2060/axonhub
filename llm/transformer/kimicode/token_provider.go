package kimicode

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

// TokenProvider refreshes Kimi Code credentials using the provider's device
// identity and strict token bundle validation.
type TokenProvider struct {
	httpClient  *httpclient.HttpClient
	oauthHost   string
	identity    Identity
	onRefreshed func(context.Context, *oauth.OAuthCredentials) error

	mu                 sync.RWMutex
	creds              *oauth.OAuthCredentials
	pendingPersistence *oauth.OAuthCredentials
	sf                 singleflight.Group

	autoMu     sync.Mutex
	autoCancel context.CancelFunc
}

type TokenProviderParams struct {
	Credentials *oauth.OAuthCredentials
	HTTPClient  *httpclient.HttpClient
	OAuthHost   string
	Identity    Identity
	OnRefreshed func(context.Context, *oauth.OAuthCredentials) error
}

func NewTokenProvider(params TokenProviderParams) (*TokenProvider, error) {
	if params.Credentials == nil {
		return nil, errors.New("credentials are required")
	}
	if params.Credentials.AccessToken == "" || params.Credentials.RefreshToken == "" || params.Credentials.ExpiresAt.IsZero() {
		return nil, errors.New("complete Kimi Code OAuth credentials are required")
	}
	if params.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}
	if err := ValidateIdentity(params.Identity); err != nil {
		return nil, err
	}
	return &TokenProvider{
		httpClient: params.HTTPClient, oauthHost: params.OAuthHost, identity: params.Identity,
		creds: params.Credentials, onRefreshed: params.OnRefreshed,
	}, nil
}

func (p *TokenProvider) Get(ctx context.Context) (*oauth.OAuthCredentials, error) {
	return p.ensure(ctx, false, "")
}

// ForceRefresh renews the token used by a failed upstream request. If another
// request has already replaced that access token, it reuses the newer bundle.
func (p *TokenProvider) ForceRefresh(ctx context.Context, failedAccessTokens ...string) (*oauth.OAuthCredentials, error) {
	failedAccessToken := ""
	if len(failedAccessTokens) > 0 {
		failedAccessToken = failedAccessTokens[0]
	}
	return p.ensure(ctx, true, failedAccessToken)
}

func (p *TokenProvider) ensure(ctx context.Context, force bool, failedAccessToken string) (*oauth.OAuthCredentials, error) {
	p.mu.RLock()
	current := p.creds
	pending := p.pendingPersistence
	p.mu.RUnlock()
	if current == nil {
		return nil, errors.New("credentials are nil; Kimi Code login is required")
	}
	if force && pending == nil && failedAccessToken != "" && current.AccessToken != failedAccessToken {
		return current, nil
	}
	if !force && pending == nil && !current.IsExpired(time.Now()) {
		return current, nil
	}
	result, err, _ := p.sf.Do("refresh", func() (any, error) {
		p.mu.RLock()
		active := p.creds
		pending := p.pendingPersistence
		p.mu.RUnlock()
		if active == nil {
			return nil, errors.New("credentials are nil; Kimi Code login is required")
		}
		if pending != nil {
			if err := p.persist(ctx, pending); err != nil {
				return nil, err
			}
			return pending, nil
		}
		if force && failedAccessToken != "" && active.AccessToken != failedAccessToken {
			return active, nil
		}
		if !force && !active.IsExpired(time.Now()) {
			return active, nil
		}
		refreshed, err := p.refresh(ctx, active)
		if err != nil {
			if isLoginRequired(err) {
				return nil, fmt.Errorf("Kimi Code login is required: %w", err)
			}
			return nil, err
		}
		p.mu.Lock()
		p.creds = refreshed
		p.pendingPersistence = refreshed
		p.mu.Unlock()
		if err := p.persist(ctx, refreshed); err != nil {
			return nil, err
		}
		return refreshed, nil
	})
	if err != nil {
		return nil, err
	}
	credentials, ok := result.(*oauth.OAuthCredentials)
	if !ok {
		return nil, fmt.Errorf("unexpected refreshed credential type %T", result)
	}
	return credentials, nil
}

func (p *TokenProvider) persist(ctx context.Context, credentials *oauth.OAuthCredentials) error {
	if p.onRefreshed != nil {
		if err := p.onRefreshed(ctx, credentials); err != nil {
			return fmt.Errorf("persist refreshed Kimi Code credentials: %w", err)
		}
	}
	p.mu.Lock()
	if p.pendingPersistence == credentials {
		p.pendingPersistence = nil
	}
	p.mu.Unlock()
	return nil
}

func (p *TokenProvider) refresh(ctx context.Context, current *oauth.OAuthCredentials) (*oauth.OAuthCredentials, error) {
	if current.RefreshToken == "" {
		return nil, errors.New("Kimi Code refresh_token is empty; login is required")
	}
	var lastErr error
	for attempt := range 3 {
		form := url.Values{"client_id": {ClientID}, "grant_type": {"refresh_token"}, "refresh_token": {current.RefreshToken}}
		request, err := makeOAuthRequest(http.MethodPost, oauthURL(p.oauthHost, TokenPath), form, p.identity)
		if err != nil {
			return nil, fmt.Errorf("build Kimi Code token refresh request: %w", err)
		}
		response, err := p.httpClient.Do(ctx, request)
		if err == nil {
			refreshed, parseErr := parseTokenWithRefreshToken(response.Body, current.RefreshToken)
			if parseErr == nil {
				refreshed.KimiCode = current.KimiCode
				return refreshed, nil
			}
			if isLoginRequired(parseErr) {
				return nil, parseErr
			}
			lastErr = parseErr
		} else {
			_, parsed := parseOAuthHTTPErrorFromError(err)
			if parsed != nil && isLoginRequired(parsed) {
				return nil, parsed
			}
			if !isRetryableRefreshError(err) {
				return nil, err
			}
			lastErr = err
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(1<<attempt) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil, fmt.Errorf("Kimi Code token refresh failed after retries: %w", lastErr)
}

func parseOAuthHTTPErrorFromError(err error) (*httpclient.Error, error) {
	var httpErr *httpclient.Error
	if !errors.As(err, &httpErr) {
		return nil, err
	}
	_, providerErr := parseOAuthHTTPError(httpErr)
	return httpErr, providerErr
}

func isRetryableRefreshError(err error) bool {
	var httpErr *httpclient.Error
	if !errors.As(err, &httpErr) {
		return true // transport failure
	}
	switch httpErr.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (p *TokenProvider) StartAutoRefresh(ctx context.Context, options oauth.AutoRefreshOptions) {
	refreshBefore := options.RefreshBefore
	if refreshBefore <= 0 {
		refreshBefore = 5 * time.Minute
	}
	interval := options.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	p.autoMu.Lock()
	if p.autoCancel != nil {
		p.autoMu.Unlock()
		return
	}
	autoCtx, cancel := context.WithCancel(ctx)
	p.autoCancel = cancel
	p.autoMu.Unlock()
	go p.runAutoRefresh(autoCtx, refreshBefore, interval)
}

func (p *TokenProvider) runAutoRefresh(ctx context.Context, refreshBefore, interval time.Duration) {
	defer func() {
		if recover() != nil {
			// A background refresh must never bring down the channel runtime.
		}
	}()
	if !p.runAutoRefreshOnce(ctx, refreshBefore) {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !p.runAutoRefreshOnce(ctx, refreshBefore) {
				return
			}
		}
	}
}

func (p *TokenProvider) runAutoRefreshOnce(ctx context.Context, refreshBefore time.Duration) bool {
	p.mu.RLock()
	creds := p.creds
	pending := p.pendingPersistence
	p.mu.RUnlock()
	if pending != nil {
		_, _ = p.ensure(ctx, false, "")
		return ctx.Err() == nil
	}
	if creds != nil && time.Until(creds.ExpiresAt) <= refreshBefore {
		_, _ = p.ensure(ctx, true, creds.AccessToken)
	}
	return ctx.Err() == nil
}

func (p *TokenProvider) StopAutoRefresh() {
	p.autoMu.Lock()
	cancel := p.autoCancel
	p.autoCancel = nil
	p.autoMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
