package kimicode

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

func TestForceRefreshSharesRefreshWithExpiredGet(t *testing.T) {
	var refreshCalls atomic.Int32
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != TokenPath {
			t.Errorf("refresh path = %q, want %q", r.URL.Path, TokenPath)
		}
		if r.Header.Get("X-Msh-Device-Id") != "device" || r.Header.Get("User-Agent") == "" {
			t.Errorf("missing Kimi CLI identity headers: %#v", r.Header)
		}
		form, err := url.ParseQuery(string(mustReadRequestBody(t, r)))
		if err != nil {
			t.Errorf("parse refresh form: %v", err)
		}
		if form.Get("grant_type") != "refresh_token" || form.Get("client_id") != ClientID || form.Get("refresh_token") != "old-refresh" {
			t.Errorf("refresh form = %v", form)
		}
		if refreshCalls.Add(1) == 1 {
			close(refreshStarted)
		}
		<-releaseRefresh
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(server.Close)

	var persistCalls atomic.Int32
	provider, err := NewTokenProvider(TokenProviderParams{
		Credentials: &oauth.OAuthCredentials{
			AccessToken:  "old-access",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthHost:  server.URL,
		Identity:   testIdentity(),
		OnRefreshed: func(_ context.Context, credentials *oauth.OAuthCredentials) error {
			persistCalls.Add(1)
			if credentials.AccessToken != "new-access" || credentials.RefreshToken != "new-refresh" {
				t.Errorf("persisted credentials = %#v", credentials)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)
	getErr := make(chan error, 1)
	forceErr := make(chan error, 1)
	go func() {
		defer wg.Done()
		_, err := provider.Get(ctx)
		getErr <- err
	}()
	<-refreshStarted
	go func() {
		defer wg.Done()
		_, err := provider.ForceRefresh(ctx, "old-access")
		forceErr <- err
	}()
	close(releaseRefresh)
	wg.Wait()

	if err := <-getErr; err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := <-forceErr; err != nil {
		t.Fatalf("ForceRefresh() error = %v", err)
	}
	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("refresh calls = %d, want 1", got)
	}
	if got := persistCalls.Load(); got != 1 {
		t.Fatalf("persist calls = %d, want 1", got)
	}
}

func TestRefreshPreservesRefreshTokenWhenProviderDoesNotRotateIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(server.Close)

	provider, err := NewTokenProvider(TokenProviderParams{
		Credentials: &oauth.OAuthCredentials{
			AccessToken:  "old-access",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthHost:  server.URL,
		Identity:   testIdentity(),
	})
	if err != nil {
		t.Fatal(err)
	}

	credentials, err := provider.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if credentials.AccessToken != "new-access" || credentials.RefreshToken != "old-refresh" {
		t.Fatalf("credentials = %#v", credentials)
	}
}

func TestRefreshRetriesPersistenceWithoutAnotherGrant(t *testing.T) {
	var refreshCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(server.Close)

	var persistCalls atomic.Int32
	provider, err := NewTokenProvider(TokenProviderParams{
		Credentials: &oauth.OAuthCredentials{
			AccessToken:  "old-access",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthHost:  server.URL,
		Identity:   testIdentity(),
		OnRefreshed: func(context.Context, *oauth.OAuthCredentials) error {
			if persistCalls.Add(1) == 1 {
				return errors.New("database unavailable")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := provider.Get(context.Background()); err == nil {
		t.Fatal("first Get() error = nil, want persistence error")
	}
	credentials, err := provider.Get(context.Background())
	if err != nil {
		t.Fatalf("second Get() error = %v", err)
	}
	if credentials.AccessToken != "new-access" || credentials.RefreshToken != "new-refresh" {
		t.Fatalf("credentials = %#v", credentials)
	}
	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("refresh calls = %d, want 1", got)
	}
	if got := persistCalls.Load(); got != 2 {
		t.Fatalf("persist calls = %d, want 2", got)
	}
}

func TestStartAutoRefreshImmediatelyRefreshesCredentialsInsideWindow(t *testing.T) {
	refreshed := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(server.Close)

	provider, err := NewTokenProvider(TokenProviderParams{
		Credentials: &oauth.OAuthCredentials{
			AccessToken:  "old-access",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(4 * time.Minute),
		},
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthHost:  server.URL,
		Identity:   testIdentity(),
		OnRefreshed: func(context.Context, *oauth.OAuthCredentials) error {
			refreshed <- struct{}{}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	provider.StartAutoRefresh(ctx, oauth.AutoRefreshOptions{RefreshBefore: 5 * time.Minute, Interval: time.Hour})
	defer provider.StopAutoRefresh()

	select {
	case <-refreshed:
	case <-time.After(2 * time.Second):
		t.Fatal("auto refresh did not run immediately")
	}
}

func TestForceRefreshSkipsWhenFailedTokenIsAlreadyReplaced(t *testing.T) {
	var refreshCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"unexpected","refresh_token":"unexpected","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(server.Close)

	provider, err := NewTokenProvider(TokenProviderParams{
		Credentials: &oauth.OAuthCredentials{
			AccessToken:  "current-access",
			RefreshToken: "current-refresh",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthHost:  server.URL,
		Identity:   testIdentity(),
	})
	if err != nil {
		t.Fatal(err)
	}

	credentials, err := provider.ForceRefresh(context.Background(), "stale-access")
	if err != nil {
		t.Fatal(err)
	}
	if credentials.AccessToken != "current-access" {
		t.Fatalf("access token = %q, want current-access", credentials.AccessToken)
	}
	if got := refreshCalls.Load(); got != 0 {
		t.Fatalf("refresh calls = %d, want 0", got)
	}
}

func mustReadRequestBody(t *testing.T, request *http.Request) []byte {
	t.Helper()
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	return body
}

func testIdentity() Identity {
	return Identity{
		UserAgentProduct: CLIUserAgentProduct,
		Version:          CLIVersion,
		Hostname:         "host",
		DeviceModel:      "model",
		OSVersion:        "os",
		DeviceID:         "device",
	}
}
