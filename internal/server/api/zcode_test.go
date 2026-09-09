package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/pkg/xcache"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/transformer/anthropic/zcode"
)

// zcodeCliRoundTripper serves the cli/init and cli/poll flow endpoints plus
// the coding-plan key-provisioning endpoints, and records the poll requests.
type zcodeCliRoundTripper struct {
	pollRequests atomic.Int32
}

func (rt *zcodeCliRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Body != nil {
		if _, err := io.ReadAll(request.Body); err != nil {
			return nil, err
		}
	}

	jsonResponse := func(payload string) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(payload)),
			Request:    request,
		}
	}

	switch {
	case strings.HasSuffix(request.URL.Path, zcode.CliInitPath):
		// The authorize URL carries a server-issued state the handler reuses as
		// session id; the redirect param gets rewritten to the desktop bridge.
		authorizeURL := zcode.BigModelAuthorizeURL + "?appId=" + zcode.BigModelAppID +
			"&redirect=" + url.QueryEscape(zcode.RedirectURI) + "&state=flow-state-1"
		return jsonResponse(`{"code":0,"msg":"","data":{"flow_id":"flow-1","poll_token":"poll-1","authorize_url":"` + authorizeURL + `","expires_in":300,"poll_interval_sec":0}}`), nil
	case strings.Contains(request.URL.Path, "/api/v1/oauth/cli/poll/"):
		// First poll stays pending, the next one is ready with the token
		// payload the desktop client receives on completion.
		if rt.pollRequests.Add(1) < 2 {
			return jsonResponse(`{"code":0,"msg":"","data":{"status":"pending"}}`), nil
		}
		return jsonResponse(`{"code":0,"msg":"","data":{"status":"ready","token":"jwt-1","bigmodel":{"access_token":"access-1","refresh_token":"refresh-1"},"expires_in":3600}}`), nil
	case strings.HasSuffix(request.URL.Path, "/api/v1/oauth/token"):
		// The callback path exchanges the authCode directly at the token
		// endpoint instead of polling the cli flow.
		return jsonResponse(`{"code":0,"msg":"","data":{"token":"jwt-1","bigmodel":{"access_token":"access-1","refresh_token":"refresh-1"},"expires_in":3600}}`), nil
	case strings.HasSuffix(request.URL.Path, "/api/biz/customer/getCustomerInfo"):
		// The real API returns data as an object (the default org nested inside),
		// not an array — the shape that broke the first live exchange.
		return jsonResponse(`{"code":200,"msg":"","data":{"organizationName":"默认机构","organizationId":"org-test-1","projects":[{"projectName":"默认项目","projectId":"proj-test-1"}]}}`), nil
	case strings.Contains(request.URL.Path, "/api_keys/copy/"):
		return jsonResponse(`{"code":200,"msg":"","data":{"secretKey":"secret-1"}}`), nil
	case strings.HasSuffix(request.URL.Path, "/api_keys"):
		return jsonResponse(`{"code":200,"msg":"","data":[{"name":"zcode-api-key","apiKey":"key-1"}]}`), nil
	default:
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Request:    request,
		}, nil
	}
}

func newZCodeTestHandler(rt *zcodeCliRoundTripper) (*ZCodeHandlers, *gin.Engine) {
	handler := NewZCodeHandlers(ZCodeHandlersParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		HttpClient:  httpclient.NewHttpClientWithClient(&http.Client{Transport: rt}),
	})
	router := gin.New()
	router.POST("/admin/zcode/oauth/start", handler.StartOAuth)
	router.POST("/admin/zcode/oauth/exchange", handler.Exchange)
	return handler, router
}

func TestZCodeHandlers_StartOAuth_rewrites_authorize_url(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, router := newZCodeTestHandler(&zcodeCliRoundTripper{})

	request := httptest.NewRequest(http.MethodPost, "/admin/zcode/oauth/start", bytes.NewBufferString("{}"))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response StartZCodeOAuthResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))

	parsed, err := url.Parse(response.AuthURL)
	require.NoError(t, err)
	require.Equal(t, zcode.BigModelAuthorizeURL, parsed.Scheme+"://"+parsed.Host+parsed.Path)
	require.Equal(t, zcode.BigModelAppID, parsed.Query().Get("appId"))
	// The redirect param is replaced with the desktop OAuth bridge.
	require.Equal(t, zcode.DesktopRedirectURI(zcode.EndpointOrigin(os.Getenv)), parsed.Query().Get("redirect"))
	// The server-issued state becomes the session id.
	require.Equal(t, "flow-state-1", response.SessionID)
	require.Equal(t, response.SessionID, parsed.Query().Get("state"))
}

func TestZCodeHandlers_Exchange_polls_until_ready(t *testing.T) {
	gin.SetMode(gin.TestMode)
	transport := &zcodeCliRoundTripper{}
	_, router := newZCodeTestHandler(transport)

	start := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/admin/zcode/oauth/start", bytes.NewBufferString("{}"))
	startRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(start, startRequest)
	var session StartZCodeOAuthResponse
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &session))

	// No callback URL needed — the upstream holds the tokens, exchange just
	// polls until the flow reports ready.
	payload, err := json.Marshal(ExchangeZCodeOAuthRequest{SessionID: session.SessionID})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/zcode/oauth/exchange", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response ExchangeZCodeOAuthResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Contains(t, response.Credentials, "jwt-1")
	// The bigmodel flow also provisions the two-part coding-plan key.
	require.Contains(t, response.Credentials, "key-1")
	require.Contains(t, response.Credentials, "secret-1")
	// The poll loop retried after the first pending response.
	require.GreaterOrEqual(t, transport.pollRequests.Load(), int32(2))
}

func TestZCodeHandlers_Exchange_callback_url(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, router := newZCodeTestHandler(&zcodeCliRoundTripper{})

	callback := zcode.RedirectURI + "?channel_id=google&utm_source=google&authCode=code-cb-1&state=state-cb-1"
	payload, err := json.Marshal(ExchangeZCodeOAuthRequest{SessionID: "anything", CallbackURL: callback})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/zcode/oauth/exchange", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("exchange failed with %d: %s", recorder.Code, recorder.Body.String())
	}
	var response ExchangeZCodeOAuthResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Contains(t, response.Credentials, "jwt-1")
}

func TestZCodeHandlers_Exchange_callback_url_missing_code(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, router := newZCodeTestHandler(&zcodeCliRoundTripper{})

	payload, err := json.Marshal(ExchangeZCodeOAuthRequest{SessionID: "anything", CallbackURL: zcode.RedirectURI + "?state=x"})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/zcode/oauth/exchange", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.Contains(t, recorder.Body.String(), "missing authCode")
}
