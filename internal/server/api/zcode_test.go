package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/pkg/xcache"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/transformer/anthropic/zcode"
)

// zcodeTokenRoundTripper serves the token and business-login endpoints and
// records the JSON body of each token request.
type zcodeTokenRoundTripper struct {
	tokenRequests []map[string]string
}

func (rt *zcodeTokenRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	switch request.URL.String() {
	case zcode.TokenURL:
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		rt.tokenRequests = append(rt.tokenRequests, payload)

		// BigModel login returns the zcode JWT inline as data.token plus the
		// bigmodel access/refresh pair.
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":0,"msg":"","data":{"token":"jwt-1","bigmodel":{"access_token":"access-1","refresh_token":"refresh-1"},"expires_in":3600}}`)),
		}, nil
	default:
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	}
}

func TestZCodeHandlers_StartOAuth_returns_registered_redirect_uri(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewZCodeHandlers(ZCodeHandlersParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, HttpClient: httpclient.NewHttpClient()})
	router := gin.New()
	router.POST("/admin/zcode/oauth/start", handler.StartOAuth)

	request := httptest.NewRequest(http.MethodPost, "/admin/zcode/oauth/start", bytes.NewBufferString("{}"))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response StartZCodeOAuthResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	parsed, err := url.Parse(response.AuthURL)
	require.NoError(t, err)
	// BigModel login: bigmodel.cn/login with custom appId/redirect/state params.
	require.Equal(t, zcode.BigModelAuthorizeURL, parsed.Scheme+"://"+parsed.Host+parsed.Path)
	require.Equal(t, zcode.BigModelAppID, parsed.Query().Get("appId"))
	require.Equal(t, zcode.RedirectURI, parsed.Query().Get("redirect"))
	require.Equal(t, response.SessionID, parsed.Query().Get("state"))
	require.Empty(t, parsed.Query().Get("response_type"))
	require.Empty(t, parsed.Query().Get("client_id"))
}

func TestZCodeHandlers_Exchange_replays_redirect_uri_from_callback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	transport := &zcodeTokenRoundTripper{}
	handler := NewZCodeHandlers(ZCodeHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  httpclient.NewHttpClientWithClient(&http.Client{Transport: transport}),
	})
	router := gin.New()
	router.POST("/admin/zcode/oauth/start", handler.StartOAuth)
	router.POST("/admin/zcode/oauth/exchange", handler.Exchange)

	start := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/admin/zcode/oauth/start", bytes.NewBufferString("{}"))
	startRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(start, startRequest)
	var session StartZCodeOAuthResponse
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &session))

	// The pasted callback carries the zcode:// scheme the authorize page
	// redirected to; BigModel returns the code as `authCode`.
	callback := "zcode://oauth/callback?authCode=synthetic-code&state=" + session.SessionID
	payload, err := json.Marshal(ExchangeZCodeOAuthRequest{SessionID: session.SessionID, CallbackURL: callback})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/zcode/oauth/exchange", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response ExchangeZCodeOAuthResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Contains(t, response.Credentials, "jwt-1")
	require.Equal(t, map[string]string{
		"provider":     zcode.BigModelProvider,
		"code":         "synthetic-code",
		"redirect_uri": "zcode://oauth/callback",
		"state":        session.SessionID,
	}, transport.tokenRequests[0])
}
