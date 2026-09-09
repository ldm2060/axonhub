package zcode

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
)

// The desktop client's server-side OAuth flow (3.10.2+ asar): instead of
// exchanging an authorization code locally, the client initializes a flow on
// the zcode origin, sends the user to the returned authorize URL, and polls
// until the server reports the login ready — the upstream then holds the
// tokens, so no local token-exchange POST ever runs (and the bridge page
// consuming the code on load cannot break it).
const (
	CliInitPath = "/api/v1/oauth/cli/init"
	CliPollPath = "/api/v1/oauth/cli/poll/"

	// CliFlowProvider is the only provider cli/init accepts — "zai" is
	// rejected with code 3004 invalid_flow.
	CliFlowProvider = "bigmodel"

	// CliFlowMaxWindow bounds the polling loop like the desktop client
	// (die = 300 * 1e3 ms).
	CliFlowMaxWindow = 5 * time.Minute
)

// CliFlowSession is the cli/init response payload.
type CliFlowSession struct {
	FlowID          string        `json:"flow_id"`
	PollToken       string        `json:"poll_token"`
	AuthorizeURL    string        `json:"authorize_url"`
	ExpiresAt       int64         `json:"expires_at"`
	PollIntervalSec int64         `json:"poll_interval_sec"`
	Interval        time.Duration `json:"-"`
}

// cliInitEnvelope wraps the cli/init response envelope.
type cliInitEnvelope struct {
	Code *int           `json:"code"`
	Msg  string         `json:"msg"`
	Data CliFlowSession `json:"data"`
}

// cliPollEnvelope wraps the cli/poll response envelope. On ready the payload
// carries the zcode plan JWT as Token plus the provider token sets — the same
// shape the manual token endpoint returns, so it maps onto TokenEnvelope.
type cliPollEnvelope struct {
	Code *int   `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TokenEnvelopeData

		Status string
	} `json:"data"`
}

// randomCliBearer generates the random 32-byte hex bearer the desktop client
// uses as both the init Authorization header and the returned poll_token.
func randomCliBearer() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// InitCliFlow starts a server-side OAuth flow on the zcode origin and returns
// the session (flow id, poll token, authorize URL).
func InitCliFlow(ctx context.Context, client *httpclient.HttpClient, origin string) (*CliFlowSession, error) {
	if client == nil {
		return nil, errors.New("http client is nil")
	}
	if origin == "" {
		origin = EndpointOriginProduction
	}

	bearer, err := randomCliBearer()
	if err != nil {
		return nil, fmt.Errorf("generate poll bearer: %w", err)
	}

	bodyBytes, err := json.Marshal(map[string]string{"provider": CliFlowProvider})
	if err != nil {
		return nil, fmt.Errorf("marshal cli/init request: %w", err)
	}

	headers := zcodeHeaders()
	headers.Set("Authorization", "Bearer "+bearer)

	req := &httpclient.Request{ //nolint:exhaustruct_v5 // Only HTTP plumbing fields matter for flow requests.
		Method:  http.MethodPost,
		URL:     strings.TrimSuffix(origin, "/") + CliInitPath,
		Headers: headers,
		Body:    bodyBytes,
	}

	resp, err := client.Do(ctx, req)
	if err != nil {
		return nil, wrapHttpError(err)
	}

	envelope, err := decodeJSONResponse[cliInitEnvelope](resp.Body)
	if err != nil {
		return nil, err
	}

	if envelope.Code != nil && *envelope.Code != 0 {
		return nil, fmt.Errorf("cli/init failed: code=%d msg=%s", *envelope.Code, envelope.Msg)
	}

	session := envelope.Data
	if session.FlowID == "" || session.PollToken == "" || session.AuthorizeURL == "" {
		return nil, errors.New("cli/init response missing flow_id/poll_token/authorize_url")
	}

	session.Interval = time.Duration(session.PollIntervalSec) * time.Second
	if session.Interval <= 0 {
		session.Interval = 2 * time.Second
	}

	return &session, nil
}

// PollCliFlow polls a cli flow once. The returned statuses map to the desktop
// client's machine: "pending" (keep waiting), "failed" (upstream error), and
// "ready" with the token payload.
func PollCliFlow(ctx context.Context, client *httpclient.HttpClient, origin, pollToken, flowID string) (status string, envelope *TokenEnvelope, err error) {
	if client == nil {
		return "", nil, errors.New("http client is nil")
	}
	if origin == "" {
		origin = EndpointOriginProduction
	}
	if pollToken == "" || flowID == "" {
		return "", nil, errors.New("poll token or flow id is empty")
	}

	headers := zcodeHeaders()
	headers.Set("Authorization", "Bearer "+pollToken)

	req := &httpclient.Request{ //nolint:exhaustruct_v5 // Only HTTP plumbing fields matter for flow requests.
		Method:  http.MethodGet,
		URL:     strings.TrimSuffix(origin, "/") + CliPollPath + url.PathEscape(flowID),
		Headers: headers,
	}

	resp, err := client.Do(ctx, req)
	if err != nil {
		return "", nil, wrapHttpError(err)
	}

	parsed, err := decodeJSONResponse[cliPollEnvelope](resp.Body)
	if err != nil {
		return "", nil, err
	}

	if parsed.Code != nil && *parsed.Code != 0 {
		return "", nil, fmt.Errorf("cli/poll failed: code=%d msg=%s", *parsed.Code, parsed.Msg)
	}

	switch parsed.Data.Status {
	case "pending":
		return "pending", nil, nil
	case "failed":
		return "failed", nil, errors.New("oauth flow failed upstream")
	case "ready":
		data := parsed.Data.TokenEnvelopeData
		if data.Token == "" {
			return "", nil, errors.New("cli/poll ready response missing data.token")
		}
		return "ready", &TokenEnvelope{Data: data}, nil
	default:
		return "", nil, fmt.Errorf("cli/poll returned unknown status %q", parsed.Data.Status)
	}
}
