package zcode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

// testServer spins up token + business-login endpoints and records the
// requests they serve.
type testServer struct {
	tokenRequests    []map[string]string
	businessRequests []map[string]string
	tokenResponse    string
	businessResponse string
}

func newTestServer(t *testing.T, ts *testServer) (tokenURL, businessURL string) {
	t.Helper()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req map[string]string
		require.NoError(t, json.Unmarshal(body, &req))
		ts.tokenRequests = append(ts.tokenRequests, req)

		require.Equal(t, "ZCode/unknown", r.Header.Get("User-Agent"))
		require.Equal(t, "https://zcode.z.ai", r.Header.Get("Http-Referer"))
		require.Equal(t, "Z Code@electron", r.Header.Get("X-Title"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(ts.tokenResponse))
	}))
	t.Cleanup(tokenSrv.Close)

	businessSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req map[string]string
		require.NoError(t, json.Unmarshal(body, &req))
		ts.businessRequests = append(ts.businessRequests, req)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(ts.businessResponse))
	}))
	t.Cleanup(businessSrv.Close)

	return tokenSrv.URL, businessSrv.URL
}

func makeJWT(exp time.Time) string {
	payload, err := json.Marshal(map[string]any{"exp": exp.Unix()})
	if err != nil {
		panic(err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func TestExchangeFullFlow(t *testing.T) {
	t.Parallel()

	ts := &testServer{
		tokenResponse:    `{"code":0,"msg":"","data":{"zai":{"access_token":"access-1","refresh_token":"refresh-1"},"expires_in":3600}}`,
		businessResponse: `{"success":true,"data":{"access_token":"jwt-1"}}`,
	}
	tokenURL, businessURL := newTestServer(t, ts)

	provider := NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(http.DefaultClient),
	})
	provider.tokenURL = tokenURL
	provider.businessURL = businessURL

	ctx := context.Background()
	creds, err := provider.Exchange(ctx, ExchangeParams{Code: "code-1", State: "state-1"})
	require.NoError(t, err)

	require.Equal(t, "access-1", creds.AccessToken)
	require.Equal(t, "refresh-1", creds.RefreshToken)
	require.Equal(t, ClientID, creds.ClientID)
	require.Equal(t, "jwt-1", creds.ZCode.BusinessJWT)
	require.True(t, creds.ExpiresAt.After(time.Now().Add(30*time.Minute)))

	require.Equal(t, map[string]string{
		"provider":     Provider,
		"code":         "code-1",
		"redirect_uri": RedirectURI,
		"state":        "state-1",
	}, ts.tokenRequests[0])
	require.Equal(t, map[string]string{"token": "access-1"}, ts.businessRequests[0])

	// Get returns the business JWT as the inference credential.
	got, err := provider.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "jwt-1", got.AccessToken)
	require.Equal(t, "jwt-1", got.ZCode.BusinessJWT) // metadata survives the copy
}

func TestExchangeRejectsBusinessEnvelopeError(t *testing.T) {
	t.Parallel()

	ts := &testServer{
		tokenResponse: `{"code":1001,"msg":"invalid code","data":{}}`,
	}
	tokenURL, businessURL := newTestServer(t, ts)

	provider := NewTokenProvider(TokenProviderParams{HTTPClient: httpclient.NewHttpClientWithClient(http.DefaultClient)})
	provider.tokenURL = tokenURL
	provider.businessURL = businessURL

	_, err := provider.Exchange(context.Background(), ExchangeParams{Code: "bad"})
	require.ErrorContains(t, err, "code=1001")
	require.Empty(t, ts.businessRequests)
}

func TestGetRefreshesWhenExpired(t *testing.T) {
	t.Parallel()

	ts := &testServer{
		// The refresh response omits refresh_token, so it must be preserved.
		tokenResponse:    `{"data":{"zai":{"access_token":"access-2"},"expires_in":3600}}`,
		businessResponse: `{"success":true,"data":{"access_token":"jwt-2"}}`,
	}
	tokenURL, businessURL := newTestServer(t, ts)

	provider := NewTokenProvider(TokenProviderParams{
		Credentials: &oauth.OAuthCredentials{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(-time.Minute),
			ZCode:        &oauth.ZCodeMetadata{BusinessJWT: makeJWT(time.Now().Add(-time.Minute))},
		},
		HTTPClient: httpclient.NewHttpClientWithClient(http.DefaultClient),
	})
	provider.tokenURL = tokenURL
	provider.businessURL = businessURL

	var persisted *oauth.OAuthCredentials
	provider.onRefreshed = func(_ context.Context, refreshed *oauth.OAuthCredentials) error {
		persisted = refreshed
		return nil
	}

	got, err := provider.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "jwt-2", got.AccessToken)
	require.Equal(t, "refresh-1", got.RefreshToken)

	require.Equal(t, map[string]string{
		"provider":      Provider,
		"grant_type":    "refresh_token",
		"refresh_token": "refresh-1",
	}, ts.tokenRequests[0])
	require.Equal(t, map[string]string{"token": "access-2"}, ts.businessRequests[0])

	require.NotNil(t, persisted)
	require.Equal(t, "jwt-2", persisted.ZCode.BusinessJWT)
}

func TestGetRefreshesWhenJWTExpiresEarly(t *testing.T) {
	t.Parallel()

	ts := &testServer{
		tokenResponse:    `{"data":{"zai":{"access_token":"access-2","refresh_token":"refresh-2"},"expires_in":3600}}`,
		businessResponse: `{"success":true,"data":{"accessToken":"jwt-2"}}`,
	}
	tokenURL, businessURL := newTestServer(t, ts)

	// OAuth token still valid for an hour, but the JWT expired a minute ago:
	// the JWT exp is the effective deadline, so Get must refresh.
	provider := NewTokenProvider(TokenProviderParams{
		Credentials: &oauth.OAuthCredentials{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(time.Hour),
			ZCode:        &oauth.ZCodeMetadata{BusinessJWT: makeJWT(time.Now().Add(-time.Minute))},
		},
		HTTPClient: httpclient.NewHttpClientWithClient(http.DefaultClient),
	})
	provider.tokenURL = tokenURL
	provider.businessURL = businessURL

	got, err := provider.Get(context.Background())
	require.NoError(t, err)
	// camelCase access token key is also accepted.
	require.Equal(t, "jwt-2", got.AccessToken)
}

func TestGetSkipsRefreshWhenFresh(t *testing.T) {
	t.Parallel()

	provider := NewTokenProvider(TokenProviderParams{
		Credentials: &oauth.OAuthCredentials{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(time.Hour),
			ZCode:        &oauth.ZCodeMetadata{BusinessJWT: "jwt-1"},
		},
	})

	got, err := provider.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "jwt-1", got.AccessToken)
	require.Equal(t, "refresh-1", got.RefreshToken)
}

func TestParseCredentialsRoundTrip(t *testing.T) {
	t.Parallel()

	creds := &oauth.OAuthCredentials{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		ExpiresAt:    time.Now().Add(time.Hour),
		ZCode:        &oauth.ZCodeMetadata{BusinessJWT: "jwt-1"},
	}

	encoded, err := creds.ToJSON()
	require.NoError(t, err)

	parsed, err := oauth.ParseCredentialsJSON(encoded)
	require.NoError(t, err)
	require.Equal(t, "jwt-1", parsed.ZCode.BusinessJWT)
}
