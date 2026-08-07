package biz

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
)

type turnstileRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f turnstileRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func setupTurnstileTestService(t *testing.T, publicURL string) (*TurnstileService, *SystemService, *ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	service := newTurnstileService(systemService, &http.Client{}, publicURL)

	return service, systemService, client, ctx
}

func storeTurnstileSettings(t *testing.T, systemService *SystemService, ctx context.Context, value string) {
	t.Helper()
	require.NoError(t, systemService.setSystemValue(ctx, SystemKeyTurnstileSettings, value))
}

func setTurnstileResponse(service *TurnstileService, status int, body string, inspect func(*http.Request)) {
	service.httpClient = &http.Client{
		Transport: turnstileRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if inspect != nil {
				inspect(req)
			}

			return &http.Response{
				StatusCode: status,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		}),
	}
}

func TestTurnstileSettingsMissingDefaultsDisabled(t *testing.T) {
	service, systemService, client, ctx := setupTurnstileTestService(t, "")
	defer client.Close()

	settings, err := systemService.TurnstileSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, &StoredTurnstileSettings{}, settings)
	require.NoError(t, service.Verify(ctx, "", TurnstileActionSignIn))

	config, err := service.PublicConfig(ctx)
	require.NoError(t, err)
	require.False(t, config.Enabled)
	require.Empty(t, config.SiteKey)
	require.Equal(t, TurnstileActionSignIn, config.Actions.SignIn)
	require.Equal(t, TurnstileActionSignUpCode, config.Actions.SignUpCode)
	require.Equal(t, TurnstileActionSignUp, config.Actions.SignUp)
}

func TestTurnstileSettingsRejectMalformedJSON(t *testing.T) {
	service, systemService, client, ctx := setupTurnstileTestService(t, "")
	defer client.Close()
	storeTurnstileSettings(t, systemService, ctx, `{not-json`)

	_, err := systemService.TurnstileSettings(ctx)
	require.Error(t, err)

	err = service.Verify(ctx, "token", TurnstileActionSignIn)
	require.ErrorIs(t, err, ErrTurnstileUnavailable)

	_, err = service.PublicConfig(ctx)
	require.ErrorIs(t, err, ErrTurnstileUnavailable)
}

func TestUpdateTurnstileSettingsValidationAndSecretPreservation(t *testing.T) {
	_, systemService, client, ctx := setupTurnstileTestService(t, "")
	defer client.Close()

	_, err := systemService.UpdateTurnstileSettings(ctx, UpdateStoredTurnstileSettings{
		Enabled: true,
	})
	require.EqualError(t, err, "turnstile site key is required when enabled")

	_, err = systemService.UpdateTurnstileSettings(ctx, UpdateStoredTurnstileSettings{
		Enabled: true,
		SiteKey: "site-key",
	})
	require.EqualError(t, err, "turnstile secret key is required when enabled")

	settings, err := systemService.UpdateTurnstileSettings(ctx, UpdateStoredTurnstileSettings{
		Enabled:   true,
		SiteKey:   " site-key ",
		SecretKey: " original-secret ",
	})
	require.NoError(t, err)
	require.Equal(t, &StoredTurnstileSettings{
		Enabled:   true,
		SiteKey:   "site-key",
		SecretKey: "original-secret",
	}, settings)

	settings, err = systemService.UpdateTurnstileSettings(ctx, UpdateStoredTurnstileSettings{
		Enabled:   true,
		SiteKey:   " replacement-site-key ",
		SecretKey: "   ",
	})
	require.NoError(t, err)
	require.Equal(t, "replacement-site-key", settings.SiteKey)
	require.Equal(t, "original-secret", settings.SecretKey)

	stored, err := systemService.TurnstileSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, settings, stored)
}

func TestTurnstilePublicConfigFailsClosedForIncompleteEnabledSettings(t *testing.T) {
	service, systemService, client, ctx := setupTurnstileTestService(t, "")
	defer client.Close()
	storeTurnstileSettings(t, systemService, ctx, `{"enabled":true,"site_key":"site-key"}`)

	_, err := service.PublicConfig(ctx)
	require.ErrorIs(t, err, ErrTurnstileUnavailable)

	err = service.Verify(ctx, "token", TurnstileActionSignIn)
	require.ErrorIs(t, err, ErrTurnstileUnavailable)
}

func TestTurnstileVerifyValidatesRequestAndResponse(t *testing.T) {
	service, systemService, client, ctx := setupTurnstileTestService(t, "https://AUTH.Example.com:8443/app")
	defer client.Close()
	storeTurnstileSettings(t, systemService, ctx, `{"enabled":true,"site_key":"site-key","secret_key":"server-secret"}`)

	setTurnstileResponse(service, http.StatusOK, `{"success":true,"hostname":"auth.example.com.","action":"signin"}`, func(req *http.Request) {
		require.Equal(t, turnstileSiteverifyURL, req.URL.String())
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		require.Equal(t, "application/json", req.Header.Get("Accept"))

		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		require.Equal(t, "server-secret", form.Get("secret"))
		require.Equal(t, "challenge-token", form.Get("response"))
		require.Empty(t, form.Get("remoteip"))
		require.Len(t, form, 2)
	})

	require.NoError(t, service.Verify(ctx, " challenge-token ", TurnstileActionSignIn))
}

func TestTurnstileVerifyRejectsMissingAndOversizedTokensWithoutRequest(t *testing.T) {
	service, systemService, client, ctx := setupTurnstileTestService(t, "")
	defer client.Close()
	storeTurnstileSettings(t, systemService, ctx, `{"enabled":true,"site_key":"site-key","secret_key":"server-secret"}`)

	called := false
	service.httpClient = &http.Client{Transport: turnstileRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("unexpected request")
	})}

	for _, token := range []string{"", "   ", strings.Repeat("x", turnstileMaxTokenBytes+1)} {
		err := service.Verify(ctx, token, TurnstileActionSignIn)
		require.ErrorIs(t, err, ErrTurnstileInvalid)
	}
	require.False(t, called)
}

func TestTurnstileVerifyClassifiesProviderFailures(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		targetErr error
	}{
		{name: "missing response", body: `{"success":false,"error-codes":["missing-input-response"]}`, targetErr: ErrTurnstileInvalid},
		{name: "invalid response", body: `{"success":false,"error-codes":["invalid-input-response"]}`, targetErr: ErrTurnstileInvalid},
		{name: "expired or duplicate", body: `{"success":false,"error-codes":["timeout-or-duplicate"]}`, targetErr: ErrTurnstileInvalid},
		{name: "missing secret", body: `{"success":false,"error-codes":["missing-input-secret"]}`, targetErr: ErrTurnstileUnavailable},
		{name: "invalid secret", body: `{"success":false,"error-codes":["invalid-input-secret"]}`, targetErr: ErrTurnstileUnavailable},
		{name: "internal error", body: `{"success":false,"error-codes":["internal-error"]}`, targetErr: ErrTurnstileUnavailable},
		{name: "bad request", body: `{"success":false,"error-codes":["bad-request"]}`, targetErr: ErrTurnstileUnavailable},
		{name: "unknown code", body: `{"success":false,"error-codes":["future-error"]}`, targetErr: ErrTurnstileUnavailable},
		{name: "mixed codes", body: `{"success":false,"error-codes":["invalid-input-response","internal-error"]}`, targetErr: ErrTurnstileUnavailable},
		{name: "no code", body: `{"success":false}`, targetErr: ErrTurnstileUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, systemService, client, ctx := setupTurnstileTestService(t, "")
			defer client.Close()
			storeTurnstileSettings(t, systemService, ctx, `{"enabled":true,"site_key":"site-key","secret_key":"server-secret"}`)
			setTurnstileResponse(service, http.StatusOK, tt.body, nil)

			err := service.Verify(ctx, "token", TurnstileActionSignIn)
			require.ErrorIs(t, err, tt.targetErr)
		})
	}
}

func TestTurnstileVerifyRejectsWrongActionAndHostname(t *testing.T) {
	t.Run("wrong action", func(t *testing.T) {
		service, systemService, client, ctx := setupTurnstileTestService(t, "")
		defer client.Close()
		storeTurnstileSettings(t, systemService, ctx, `{"enabled":true,"site_key":"site-key","secret_key":"server-secret"}`)
		setTurnstileResponse(service, http.StatusOK, `{"success":true,"hostname":"other.example.com","action":"signup"}`, nil)

		err := service.Verify(ctx, "token", TurnstileActionSignIn)
		require.ErrorIs(t, err, ErrTurnstileInvalid)
	})

	t.Run("canonical hostname mismatch", func(t *testing.T) {
		service, systemService, client, ctx := setupTurnstileTestService(t, "https://auth.example.com")
		defer client.Close()
		storeTurnstileSettings(t, systemService, ctx, `{"enabled":true,"site_key":"site-key","secret_key":"server-secret"}`)
		setTurnstileResponse(service, http.StatusOK, `{"success":true,"hostname":"evil.example.com","action":"signin"}`, nil)

		err := service.Verify(ctx, "token", TurnstileActionSignIn)
		require.ErrorIs(t, err, ErrTurnstileInvalid)
	})
}

func TestTurnstileVerifyFailsClosedOnProviderResponseFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "non success status", status: http.StatusBadGateway, body: `{"success":true,"action":"signin"}`},
		{name: "malformed json", status: http.StatusOK, body: `{not-json`},
		{name: "oversized response", status: http.StatusOK, body: strings.Repeat("x", turnstileMaxResponseBodySize+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, systemService, client, ctx := setupTurnstileTestService(t, "")
			defer client.Close()
			storeTurnstileSettings(t, systemService, ctx, `{"enabled":true,"site_key":"site-key","secret_key":"server-secret"}`)
			setTurnstileResponse(service, tt.status, tt.body, nil)

			err := service.Verify(ctx, "token", TurnstileActionSignIn)
			require.ErrorIs(t, err, ErrTurnstileUnavailable)
		})
	}
}

func TestTurnstileVerifyFailsClosedOnTransportAndReadErrors(t *testing.T) {
	t.Run("transport", func(t *testing.T) {
		service, systemService, client, ctx := setupTurnstileTestService(t, "")
		defer client.Close()
		storeTurnstileSettings(t, systemService, ctx, `{"enabled":true,"site_key":"site-key","secret_key":"server-secret"}`)
		service.httpClient = &http.Client{Transport: turnstileRoundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		})}

		err := service.Verify(ctx, "token", TurnstileActionSignIn)
		require.ErrorIs(t, err, ErrTurnstileUnavailable)
	})

	t.Run("body read", func(t *testing.T) {
		service, systemService, client, ctx := setupTurnstileTestService(t, "")
		defer client.Close()
		storeTurnstileSettings(t, systemService, ctx, `{"enabled":true,"site_key":"site-key","secret_key":"server-secret"}`)
		service.httpClient = &http.Client{Transport: turnstileRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(errorReader{}),
				Request:    req,
			}, nil
		})}

		err := service.Verify(ctx, "token", TurnstileActionSignIn)
		require.ErrorIs(t, err, ErrTurnstileUnavailable)
	})
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestTurnstileHTTPClientRejectsRedirects(t *testing.T) {
	client := newTurnstileHTTPClient()
	err := client.CheckRedirect(nil, nil)
	require.ErrorIs(t, err, http.ErrUseLastResponse)
	require.Equal(t, turnstileRequestTimeout, client.Timeout)
}

func TestCanonicalHostname(t *testing.T) {
	require.Equal(t, "auth.example.com", canonicalHostname(" HTTPS://AUTH.Example.com.:8443/path "))
	require.Empty(t, canonicalHostname("auth.example.com"))
	require.Empty(t, canonicalHostname("://invalid"))
}
