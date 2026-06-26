# WebSocket Inbound Streaming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add inbound WebSocket (ws/wss) transport to the streaming model endpoints (mirroring the outbound Responses WS connection model) plus an admin-configurable idle keepalive.

**Architecture:** A new `ws.go` in `internal/server/api` upgrades inbound connections on the same paths as the streaming POSTs, reads each request as a WS message, feeds a synthetic `*http.Request` (built from the upgrade request + message body) into the existing `ChatCompletionHandlers` orchestrator, and writes the response back as WS messages — typed JSON events for Responses (symmetric with the outbound read side) and exact SSE-event bytes per message for chat/Anthropic/Gemini. A PING-frame keepalive runs per connection on a configurable interval stored in a new `StreamingSettings` system setting exposed in the admin UI.

**Tech Stack:** Go 1.26, Gin, gorilla/websocket (already a dependency), gin-contrib/sse, gqlgen, Ent, React 19 + TanStack Query, Tailwind, i18n (en/zh-CN).

## Global Constraints

- `github.com/gorilla/websocket` is already in `go.mod` (used by `llm/transformer/openai/responses`); do NOT add a new WS module.
- WebSocket handshake auth is header-only (`Authorization` / `X-API-Key` / `X-Goog-Api-Key`) via the existing route-group middleware; do NOT add query-param auth.
- Regenerate GraphQL code with `make generate` (runs `cd internal/server/gql && go generate`) whenever `system.graphql` or `gqlgen.yml` changes.
- Every manually started goroutine installs a top-level `defer recover()` that logs before exit.
- Use `github.com/samber/lo` for pointer/slice helpers; structured logging via `internal/log`; propagate `context.Context`.
- Before each commit: `go build ./...`, `cd llm && go build ./...`, lint (`golangci-lint run --timeout 10m --max-same-issues 50 ./...`), `go test ./...`. After backend changes that affect the running server, rebuild `axonhub.exe`, restart, and verify before committing.
- Frontend: use `pnpm`; update GraphQL query, mutation, and TypeScript types in the same change; add both `en` and `zh-CN` locale entries.

---

## File Structure

**Backend:**
- `internal/server/biz/system.go` — add `StreamingSettings` (struct, key, default, get/set, normalize).
- `internal/server/biz/system_test.go` — round-trip + normalize tests.
- `internal/server/gql/system.graphql` — `StreamingSettings` type, `UpdateStreamingSettingsInput` input, query + mutation.
- `internal/server/gql/gqlgen.yml` — bind both GraphQL types to `biz.StreamingSettings`.
- `internal/server/gql/generated.go` — regenerated (`make generate`).
- `internal/server/gql/system.resolvers.go` — `StreamingSettings` query + `UpdateStreamingSettings` mutation resolvers.
- `internal/server/api/ws.go` (new) — upgrader, frame modes, request prep, synthetic request builder, WS writers, keepalive, `ChatCompletionWebSocket` method.
- `internal/server/api/ws_test.go` (new) — unit tests for the above.
- `internal/server/api/chat.go` — no behavioral change required (reuse `nextStreamEvent`, `newUpstreamErrorStream`, `FormatStreamError`, `transformOrchestratorError`).
- `internal/server/api/openai.go` — add thin WS handler methods on `OpenAIHandlers`.
- `internal/server/api/anthropic.go` — add thin WS handler method on `AnthropicHandlers`.
- `internal/server/api/gemini.go` — add thin WS handler method on `GeminiHandlers`.
- `internal/server/routes.go` — register `GET` WS handlers on the streaming endpoints.

**Frontend:**
- `frontend/src/features/system/data/system.ts` — query, mutation, types, hooks.
- `frontend/src/features/system/components/streaming-settings.tsx` (new) — the Streaming card.
- `frontend/src/features/system/components/tabs.tsx` — register the `streaming` tab.
- `frontend/src/routes/_authenticated/admin/system/index.tsx` — add `streaming` to `SystemTabKey`.
- `frontend/src/locales/en/system.json`, `frontend/src/locales/zh-CN/system.json` — copy.

---

## Task 1: `StreamingSettings` biz layer

**Files:**
- Modify: `internal/server/biz/system.go` (add near the `RetryPolicy` block, ~line 380, and near `SetRetryPolicy` ~line 1128).
- Test: `internal/server/biz/system_test.go`.

**Interfaces:**
- Produces: `biz.StreamingSettings` struct with `WebSocketKeepaliveIntervalSeconds int`; `(*SystemService).StreamingSettings(ctx) (*StreamingSettings, error)`; `(*SystemService).SetStreamingSettings(ctx, *StreamingSettings) error`; `normalizeStreamingSettings(*StreamingSettings)`; constant `SystemKeyStreamingSettings = "streaming_settings"`.

- [ ] **Step 1: Write the failing test**

Append to `internal/server/biz/system_test.go`:

```go
func TestStreamingSettings_Normalize(t *testing.T) {
	t.Parallel()

	settings := &StreamingSettings{WebSocketKeepaliveIntervalSeconds: -5}
	normalizeStreamingSettings(settings)
	require.Equal(t, 0, settings.WebSocketKeepaliveIntervalSeconds)

	settings = &StreamingSettings{WebSocketKeepaliveIntervalSeconds: 15}
	normalizeStreamingSettings(settings)
	require.Equal(t, 15, settings.WebSocketKeepaliveIntervalSeconds)
}

func TestStreamingSettings_MarshalRoundTrip(t *testing.T) {
	t.Parallel()

	settings := &StreamingSettings{WebSocketKeepaliveIntervalSeconds: 12}
	data, err := json.Marshal(settings)
	require.NoError(t, err)

	var got StreamingSettings
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, 12, got.WebSocketKeepaliveIntervalSeconds)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/biz/ -run TestStreamingSettings -v`
Expected: FAIL / compile error (`StreamingSettings` undefined).

- [ ] **Step 3: Write minimal implementation**

In `internal/server/biz/system.go`, add after the `UpstreamErrorPolicy` struct (around line 461, before `AutoDisableChannel`):

```go
// StreamingSettings configures inbound WebSocket streaming behavior.
type StreamingSettings struct {
	// WebSocketKeepaliveIntervalSeconds controls how often a PING control frame is
	// sent on an idle inbound WebSocket stream to prevent proxies / load balancers
	// from closing long-running connections. 0 disables keepalive.
	WebSocketKeepaliveIntervalSeconds int `json:"web_socket_keepalive_interval_seconds"`
}
```

Add to the system-key `const` block (near `SystemKeyRetryPolicy`, ~line 65):

```go
	// SystemKeyStreamingSettings is the key used to store the streaming settings.
	SystemKeyStreamingSettings = "streaming_settings"
```

Add a default and the accessors. Place near `RetryPolicyOrDefault` (~line 1117), after `SetRetryPolicy`:

```go
var defaultStreamingSettings = StreamingSettings{}

// StreamingSettings retrieves the streaming settings configuration.
func (s *SystemService) StreamingSettings(ctx context.Context) (*StreamingSettings, error) {
	value, err := s.getSystemValue(ctx, SystemKeyStreamingSettings)
	if err != nil {
		if ent.IsNotFound(err) {
			settings := defaultStreamingSettings
			normalizeStreamingSettings(&settings)
			return &settings, nil
		}

		return nil, fmt.Errorf("failed to get streaming settings: %w", err)
	}

	var settings StreamingSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal streaming settings: %w", err)
	}

	normalizeStreamingSettings(&settings)

	return &settings, nil
}

// SetStreamingSettings sets the streaming settings configuration.
func (s *SystemService) SetStreamingSettings(ctx context.Context, settings *StreamingSettings) error {
	normalizeStreamingSettings(settings)

	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal streaming settings: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyStreamingSettings, string(jsonBytes))
}

func normalizeStreamingSettings(settings *StreamingSettings) {
	if settings == nil {
		return
	}

	if settings.WebSocketKeepaliveIntervalSeconds < 0 {
		settings.WebSocketKeepaliveIntervalSeconds = 0
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/biz/ -run TestStreamingSettings -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/biz/system.go internal/server/biz/system_test.go
git commit -m "feat(biz): add StreamingSettings system setting"
```

---

## Task 2: `StreamingSettings` GraphQL surface

**Files:**
- Modify: `internal/server/gql/system.graphql` (add near the `RetryPolicy` type ~line 145 and `updateRetryPolicy` ~line 435).
- Modify: `internal/server/gql/gqlgen.yml` (add near the `RetryPolicy` binding ~line 204).
- Modify: `internal/server/gql/system.resolvers.go`.
- Regenerate: `internal/server/gql/generated.go` via `make generate`.

**Interfaces:**
- Consumes: `biz.StreamingSettings`, `(*SystemService).StreamingSettings` / `SetStreamingSettings` (Task 1).
- Produces: GraphQL `streamingSettings` query and `updateStreamingSettings(input: UpdateStreamingSettingsInput!)` mutation, bound to `biz.StreamingSettings`.

- [ ] **Step 1: Add the GraphQL schema types**

In `internal/server/gql/system.graphql`, immediately after the `RetryPolicy` type's closing brace (~line 178, before `input UpdateRetryPolicyInput`):

```graphql
type StreamingSettings {
  webSocketKeepaliveIntervalSeconds: Int!
}

input UpdateStreamingSettingsInput {
  webSocketKeepaliveIntervalSeconds: Int
}
```

Then add to the `Query` type (next to `retryPolicy: RetryPolicy!`):

```graphql
  streamingSettings: StreamingSettings!
```

And to the `Mutation` type (next to `updateRetryPolicy(input: UpdateRetryPolicyInput!): Boolean!`):

```graphql
  updateStreamingSettings(input: UpdateStreamingSettingsInput!): Boolean!
```

- [ ] **Step 2: Bind the types in gqlgen.yml**

In `internal/server/gql/gqlgen.yml`, immediately after the `UpdateRetryPolicyInput` block (~line 209):

```yaml
  StreamingSettings:
    model:
      - github.com/ldm2060/axonhub/internal/server/biz.StreamingSettings
  UpdateStreamingSettingsInput:
    model:
      - github.com/ldm2060/axonhub/internal/server/biz.StreamingSettings
```

- [ ] **Step 3: Regenerate**

Run: `make generate`
Expected: `internal/server/gql/generated.go` is updated to contain `StreamingSettings`, `UpdateStreamingSettingsInput`, the `streamingSettings` query resolver stub, and the `updateStreamingSettings` mutation resolver stub.

- [ ] **Step 4: Implement the resolvers**

In `internal/server/gql/system.resolvers.go`, add (near `RetryPolicy`/`UpdateRetryPolicy`, ~line 464/66):

```go
// StreamingSettings is the resolver for the streamingSettings field.
func (r *queryResolver) StreamingSettings(ctx context.Context) (*biz.StreamingSettings, error) {
	return r.systemService.StreamingSettings(ctx)
}

// UpdateStreamingSettings is the resolver for the updateStreamingSettings field.
func (r *mutationResolver) UpdateStreamingSettings(ctx context.Context, input biz.StreamingSettings) (bool, error) {
	if err := r.systemService.SetStreamingSettings(ctx, &input); err != nil {
		return false, fmt.Errorf("failed to update streaming settings: %w", err)
	}

	return true, nil
}
```

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./internal/server/gql/... ./internal/server/biz/...`
Expected: build succeeds, tests pass (the generated code compiles with the new resolvers).

- [ ] **Step 6: Commit**

```bash
git add internal/server/gql/system.graphql internal/server/gql/gqlgen.yml internal/server/gql/generated.go internal/server/gql/system.resolvers.go
git commit -m "feat(gql): expose StreamingSettings query and mutation"
```

---

## Task 3: WebSocket core handler (`ws.go`)

**Files:**
- Create: `internal/server/api/ws.go`.
- Create: `internal/server/api/ws_test.go`.

**Interfaces:**
- Consumes: `ChatCompletionHandlers.processor()`, `ChatCompletionHandlers.systemService()`, `ChatCompletionHandlers.StreamIdleTimeout`, `ChatCompletionHandlers.ChatCompletionOrchestrator`; package helpers `nextStreamEvent`, `newUpstreamErrorStream`, `FormatStreamError`, `transformOrchestratorError`; `httpclient.ReadHTTPRequest`; `biz.StreamingSettings` (Task 1).
- Produces: `type WSFrameMode` with `WSFrameSSEBytes` / `WSFrameResponsesEvents`; `(*ChatCompletionHandlers).ChatCompletionWebSocket(c *gin.Context, mode WSFrameMode)`; internal helpers `prepareWSRequest`, `buildWSGenericRequest`, `writeWSResponse`, `writeWSStream`, `startWSKeepalive`.

- [ ] **Step 1: Write the failing tests**

Create `internal/server/api/ws_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/server/orchestrator"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

type fakeStream struct {
	events []*httpclient.StreamEvent
	idx    int
}

func (s *fakeStream) Next() bool {
	s.idx++
	return s.idx <= len(s.events)
}

func (s *fakeStream) Current() *httpclient.StreamEvent {
	if s.idx >= 1 && s.idx <= len(s.events) {
		return s.events[s.idx-1]
	}
	return nil
}

func (s *fakeStream) Err() error  { return nil }
func (s *fakeStream) Close() error { return nil }

func wsURL(server *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + path
}

func TestPrepareWSRequest_ResponsesUnwrapsEnvelope(t *testing.T) {
	t.Parallel()

	out, err := prepareWSRequest(WSFrameResponsesEvents, []byte(`{"type":"response.create","model":"gpt","input":[]}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gpt","input":[],"stream":true}`, string(out))
}

func TestPrepareWSRequest_SSEBytesPassthrough(t *testing.T) {
	t.Parallel()

	out, err := prepareWSRequest(WSFrameSSEBytes, []byte(`{"model":"gpt","stream":true}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gpt","stream":true}`, string(out))
}

func TestPrepareWSRequest_Empty(t *testing.T) {
	t.Parallel()

	_, err := prepareWSRequest(WSFrameSSEBytes, nil)
	require.Error(t, err)
}

func TestBuildWSGenericRequest_SetsBodyAndPOST(t *testing.T) {
	t.Parallel()

	upgrade := httptest.NewRequest(http.MethodGet, "/v1/chat/completions?x=1", strings.NewReader(""))
	upgrade.Header.Set("Authorization", "Bearer abc")

	req, err := buildWSGenericRequest(upgrade, []byte(`{"stream":true}`))
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, req.Method)
	require.Equal(t, "/v1/chat/completions", req.Path)
	require.Equal(t, "1", req.Query.Get("x"))
	require.Equal(t, `{"stream":true}`, string(req.Body))
	require.Equal(t, "Bearer abc", req.Headers.Get("Authorization"))
}

func TestWriteWSStream_SSEBytesMirrorsSSE(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		stream := &fakeStream{events: []*httpclient.StreamEvent{
			{Type: "data", Data: []byte(`{"foo":"bar"}`)},
		}}
		writeWSStream(r.Context(), conn, stream, WSFrameSSEBytes, 0)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server, ""), nil)
	require.NoError(t, err)
	defer conn.Close()

	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	var want bytes.Buffer
	require.NoError(t, sse.Encode(&want, sse.Event{Event: "data", Data: []byte(`{"foo":"bar"}`)}))
	require.Equal(t, want.String(), string(msg))
}

func TestWriteWSStream_ResponsesEventsSendsTypedJSON(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		stream := &fakeStream{events: []*httpclient.StreamEvent{
			{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","delta":"hi"}`)},
		}}
		writeWSStream(r.Context(), conn, stream, WSFrameResponsesEvents, 0)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server, ""), nil)
	require.NoError(t, err)
	defer conn.Close()

	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"response.output_text.delta","delta":"hi"}`, string(msg))
}

func TestStartWSKeepalive_SendsPing(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		stop := startWSKeepalive(r.Context(), conn, 20*time.Millisecond)
		defer stop()
		_, _, _ = conn.ReadMessage() // block until closed
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server, ""), nil)
	require.NoError(t, err)
	defer conn.Close()

	got := make(chan struct{}, 4)
	conn.SetPingHandler(func(string) error { got <- struct{}{}; return nil })

	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("did not receive PING")
	}
}

func TestStartWSKeepalive_DisabledWhenZero(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		stop := startWSKeepalive(r.Context(), conn, 0)
		defer stop()
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server, ""), nil)
	require.NoError(t, err)
	defer conn.Close()

	got := make(chan struct{}, 1)
	conn.SetPingHandler(func(string) error { got <- struct{}{}; return nil })

	select {
	case <-got:
		t.Fatal("received unexpected PING")
	case <-time.After(120 * time.Millisecond):
	}
}

type fakeProcessor struct {
	result orchestrator.ChatCompletionResult
	err    error
}

func (f fakeProcessor) Process(_ context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
	return f.result, f.err
}

func TestChatCompletionWebSocket_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handlers := &ChatCompletionHandlers{
		Processor: fakeProcessor{result: orchestrator.ChatCompletionResult{
			ChatCompletionStream: &fakeStream{events: []*httpclient.StreamEvent{
				{Type: "data", Data: []byte(`{"x":1}`)},
			}},
		}},
	}

	r := gin.New()
	r.GET("/ws", func(c *gin.Context) { handlers.ChatCompletionWebSocket(c, WSFrameSSEBytes) })
	server := httptest.NewServer(r)
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server, "/ws"), nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"model":"gpt","stream":true}`)))

	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(msg), `"x":1`)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/api/ -run 'TestPrepareWSRequest|TestBuildWSGenericRequest|TestWriteWSStream|TestStartWSKeepalive|TestChatCompletionWebSocket' -v`
Expected: FAIL / compile error (`ws.go` symbols undefined).

- [ ] **Step 3: Write `ws.go`**

Create `internal/server/api/ws.go`:

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/orchestrator"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/streams"
)

// WSFrameMode selects how stream events are framed on an inbound WebSocket.
type WSFrameMode int

const (
	// WSFrameSSEBytes sends each stream event as one WS text message containing
	// the exact SSE event bytes (event:/data:). Used by chat / Anthropic / Gemini.
	WSFrameSSEBytes WSFrameMode = iota
	// WSFrameResponsesEvents sends each event's JSON object (with its top-level
	// "type" field) as one WS text message, mirroring the outbound Responses WS.
	WSFrameResponsesEvents
)

// inboundWSUpgrader upgrades inbound model-endpoint WebSocket connections.
var inboundWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// prepareWSRequest converts an inbound WS message into the request body bytes
// expected by the pipeline for the given frame mode.
func prepareWSRequest(mode WSFrameMode, msg []byte) ([]byte, error) {
	if len(msg) == 0 {
		return nil, errors.New("request message is empty")
	}

	payload := map[string]any{}
	if err := json.Unmarshal(msg, &payload); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}

	delete(payload, "type") // strip a response.create envelope if present

	if mode == WSFrameResponsesEvents {
		// Responses WS is inherently event-streamed; force streaming on.
		payload["stream"] = true
	}

	return json.Marshal(payload)
}

// buildWSGenericRequest builds a pipeline-ready generic request from the WS
// upgrade request and the prepared body bytes. Method/path/query/headers come
// from the upgrade request; the body comes from the WS message.
func buildWSGenericRequest(upgrade *http.Request, body []byte) (*httpclient.Request, error) {
	req := upgrade.Clone(upgrade.Context())
	req.Method = http.MethodPost
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return httpclient.ReadHTTPRequest(req)
}

// writeWSResponse writes a non-streaming response as a single WS message.
func writeWSResponse(conn *websocket.Conn, resp *httpclient.Response) error {
	if resp == nil {
		return nil
	}

	return conn.WriteMessage(websocket.TextMessage, resp.Body)
}

// writeWSStream drains the stream and writes each event to the WS connection
// according to mode. It returns when the stream ends, errors out, or the
// connection is no longer writable. The stream is always closed.
func writeWSStream(ctx context.Context, conn *websocket.Conn, stream streams.Stream[*httpclient.StreamEvent], mode WSFrameMode, idleTimeout time.Duration) {
	defer func() {
		if err := stream.Close(); err != nil {
			log.Debug(ctx, "close ws stream", log.Cause(err))
		}
	}()

	for {
		result := nextStreamEvent(ctx, stream, idleTimeout)
		if result.ok {
			if err := writeWSStreamEvent(conn, result.event, mode); err != nil {
				log.Warn(ctx, "ws write failed, stopping stream", log.Cause(err))
				return
			}

			continue
		}

		if result.err != nil {
			if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
				log.Warn(ctx, "ws stream context done", log.Cause(result.err))
				return
			}

			log.Warn(ctx, "ws stream error", log.Cause(result.err))
			_ = writeWSStreamEvent(conn, &httpclient.StreamEvent{
				Type: "error",
				Data: wsErrorEventData(ctx, mode, result.err),
			}, mode)
		}

		return
	}
}

// writeWSStreamEvent encodes one stream event as a WS text message.
func writeWSStreamEvent(conn *websocket.Conn, ev *httpclient.StreamEvent, mode WSFrameMode) error {
	if ev == nil {
		return nil
	}

	switch mode {
	case WSFrameResponsesEvents:
		if len(ev.Data) == 0 {
			return nil
		}

		return conn.WriteMessage(websocket.TextMessage, ev.Data)
	default:
		var buf bytes.Buffer
		if err := sse.Encode(&buf, sse.Event{Event: ev.Type, Data: ev.Data}); err != nil {
			return err
		}

		return conn.WriteMessage(websocket.TextMessage, buf.Bytes())
	}
}

// wsErrorEventData returns the error-event payload bytes appropriate for the mode.
func wsErrorEventData(ctx context.Context, mode WSFrameMode, err error) []byte {
	if mode == WSFrameResponsesEvents {
		body, _ := json.Marshal(map[string]any{
			"type":    "error",
			"code":    "server_error",
			"message": orchestrator.ExtractErrorMessage(err),
		})
		return body
	}

	body, _ := json.Marshal(FormatStreamError(ctx, err))
	return body
}

// startWSKeepalive sends a PING control frame every interval until the returned
// stop func is called or the context is cancelled. WriteControl is safe for
// concurrent use with WriteMessage. Returns a no-op stop func when interval <= 0.
func startWSKeepalive(ctx context.Context, conn *websocket.Conn, interval time.Duration) (stop func()) {
	if interval <= 0 {
		return func() {}
	}

	done := make(chan struct{})

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Warn(ctx, "ws keepalive panic recovered", log.Any("panic", rec))
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			}
		}
	}()

	return func() { close(done) }
}

// keepaliveInterval reads the configured WS keepalive interval from system settings.
func (h *ChatCompletionHandlers) keepaliveInterval(ctx context.Context) time.Duration {
	svc := h.systemService()
	if svc == nil {
		return 0
	}

	settings, err := svc.StreamingSettings(ctx)
	if err != nil || settings == nil {
		return 0
	}

	return time.Duration(settings.WebSocketKeepaliveIntervalSeconds) * time.Second
}

// ChatCompletionWebSocket handles an inbound WebSocket connection that mirrors
// the HTTP streaming endpoint. Each inbound text message is one request; the
// response is streamed back as WS messages. The connection is kept open for
// multiple serial requests until the client disconnects.
func (h *ChatCompletionHandlers) ChatCompletionWebSocket(c *gin.Context, mode WSFrameMode) {
	conn, err := inboundWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade already wrote the HTTP error response.
		return
	}

	defer func() {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
	}()

	ctx := c.Request.Context()
	stop := startWSKeepalive(ctx, conn, h.keepaliveInterval(ctx))
	defer stop()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return // client closed or read error
		}

		if exit := h.handleWSRequest(ctx, conn, c, msg, mode); exit {
			return
		}
	}
}

// handleWSRequest processes one WS request message. It returns true if the
// connection should be closed (write failure), false to continue the loop.
func (h *ChatCompletionHandlers) handleWSRequest(ctx context.Context, conn *websocket.Conn, c *gin.Context, msg []byte, mode WSFrameMode) bool {
	body, err := prepareWSRequest(mode, msg)
	if err != nil {
		log.Warn(ctx, "ws request prepare error", log.Cause(err))
		return h.writeWSRequestError(conn, ctx, mode, err)
	}

	genericReq, err := buildWSGenericRequest(c.Request, body)
	if err != nil {
		log.Warn(ctx, "ws request build error", log.Cause(err))
		return h.writeWSRequestError(conn, ctx, mode, err)
	}

	result, err := h.processor().Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "ws process error", log.Cause(err))
		return h.writeWSRequestError(conn, ctx, mode, err)
	}

	if result.ChatCompletion != nil {
		if werr := writeWSResponse(conn, result.ChatCompletion); werr != nil {
			return true
		}

		return false
	}

	if result.ChatCompletionStream != nil {
		stream := newUpstreamErrorStream(ctx, result.ChatCompletionStream, h.systemService())
		writeWSStream(ctx, conn, stream, mode, h.StreamIdleTimeout)
		return false
	}

	// No response object and no stream: nothing to send for this request.
	return false
}

// writeWSRequestError sends a single error frame for a failed request.
// Returns true if the write failed (caller should close the connection).
func (h *ChatCompletionHandlers) writeWSRequestError(conn *websocket.Conn, ctx context.Context, mode WSFrameMode, err error) bool {
	var data []byte
	if h.ChatCompletionOrchestrator != nil {
		httpErr := transformOrchestratorError(ctx, err, h.ChatCompletionOrchestrator)
		if mode == WSFrameResponsesEvents {
			obj := map[string]any{
				"type":    "error",
				"code":    "server_error",
				"message": orchestrator.ExtractErrorMessage(err),
			}
			data, _ = json.Marshal(obj)
		} else {
			data = httpErr.Body
		}
	} else {
		data = wsErrorEventData(ctx, mode, err)
	}

	if werr := conn.WriteMessage(websocket.TextMessage, data); werr != nil {
		return true
	}

	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/api/ -run 'TestPrepareWSRequest|TestBuildWSGenericRequest|TestWriteWSStream|TestStartWSKeepalive|TestChatCompletionWebSocket' -v`
Expected: PASS for all tests.

- [ ] **Step 5: Commit**

```bash
git add internal/server/api/ws.go internal/server/api/ws_test.go
git commit -m "feat(api): add inbound WebSocket streaming handler and keepalive"
```

---

## Task 4: Route wiring and per-format handler methods

**Files:**
- Modify: `internal/server/api/openai.go` — add WS handler methods on `OpenAIHandlers`.
- Modify: `internal/server/api/anthropic.go` — add WS handler method on `AnthropicHandlers`.
- Modify: `internal/server/api/gemini.go` — add WS handler method on `GeminiHandlers`.
- Modify: `internal/server/routes.go` — register `GET` WS handlers.

**Interfaces:**
- Consumes: `(*ChatCompletionHandlers).ChatCompletionWebSocket(c, mode)` (Task 3); the per-format `*ChatCompletionHandlers` instances (`OpenAIHandlers.ChatCompletionHandlers`, `.ResponseCompletionHandlers`, `.CompactHandlers`; `AnthropicHandlers` chat handlers; `GeminiHandlers` handlers).
- Produces: `OpenAIHandlers.ChatCompletionWebSocket`, `OpenAIHandlers.CreateResponseWebSocket`, `OpenAIHandlers.CompactResponseWebSocket`; `AnthropicHandlers.CreateMessageWebSocket`; `GeminiHandlers.GenerateContentWebSocket`; registered `GET` WS routes on `/v1/chat/completions`, `/v1/responses`, `/v1/responses/compact`, `/v1/messages`, `/anthropic/v1/messages`, `/v1beta/models/*action` (+ `/gemini/:gemini-api-version/models/*action`).

- [ ] **Step 1: Add the per-format handler methods**

In `internal/server/api/openai.go`, add near the other `OpenAIHandlers` methods (e.g. after `CreateTranslation`, ~line 386):

```go
// ChatCompletionWebSocket handles GET /v1/chat/completions over WebSocket.
func (handlers *OpenAIHandlers) ChatCompletionWebSocket(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletionWebSocket(c, WSFrameSSEBytes)
}

// CreateResponseWebSocket handles GET /v1/responses over WebSocket.
func (handlers *OpenAIHandlers) CreateResponseWebSocket(c *gin.Context) {
	handlers.ResponseCompletionHandlers.ChatCompletionWebSocket(c, WSFrameResponsesEvents)
}

// CompactResponseWebSocket handles GET /v1/responses/compact over WebSocket.
func (handlers *OpenAIHandlers) CompactResponseWebSocket(c *gin.Context) {
	handlers.CompactHandlers.ChatCompletionWebSocket(c, WSFrameResponsesEvents)
}
```

In `internal/server/api/anthropic.go`, add after the existing `AnthropicHandlers` methods (the struct field is `ChatCompletionHandlers`, confirmed):

```go
// CreateMessageWebSocket handles GET /v1/messages and /anthropic/v1/messages over WebSocket.
func (handlers *AnthropicHandlers) CreateMessageWebSocket(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletionWebSocket(c, WSFrameSSEBytes)
}
```

In `internal/server/api/gemini.go`, add after `GenerateContent` (the struct field is `ChatCompletionHandlers`, confirmed):

```go
// GenerateContentWebSocket handles GET .../models/*action over WebSocket.
func (handlers *GeminiHandlers) GenerateContentWebSocket(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletionWebSocket(c, WSFrameSSEBytes)
}
```

These are one-line delegations; the WS behavior itself is covered by `TestChatCompletionWebSocket_HappyPath` from Task 3.

- [ ] **Step 2: Register the GET WS routes**

In `internal/server/routes.go`, inside the `openaiGroup := apiGroup.Group("/v1")` block (after line 209, the `/messages` POST), add:

```go
		openaiGroup.GET("/chat/completions", handlers.OpenAI.ChatCompletionWebSocket)
		openaiGroup.GET("/responses", handlers.OpenAI.CreateResponseWebSocket)
		openaiGroup.GET("/responses/compact", handlers.OpenAI.CompactResponseWebSocket)
		openaiGroup.GET("/messages", handlers.Anthropic.CreateMessageWebSocket)
```

Inside the `anthropicGroup := apiGroup.Group("/anthropic/v1")` block (after line 223), add:

```go
		anthropicGroup.GET("/messages", handlers.Anthropic.CreateMessageWebSocket)
```

Inside `registerGeminiRoutes` (after the `/models/*action` POST at line 236), add:

```go
			group.GET("/models/*action", handlers.Gemini.GenerateContentWebSocket)
```

- [ ] **Step 3: Build and run all backend tests**

Run: `go build ./... && cd llm && go build ./...`
Then: `go test ./...` and `cd llm && go test ./...`
Expected: everything compiles and passes.

- [ ] **Step 4: Lint**

Run: `golangci-lint run --timeout 10m --max-same-issues 50 ./...`
Expected: no new issues. Fix any reported issues before committing.

- [ ] **Step 5: Manual integration check**

Rebuild and restart the local server, then verify each endpoint with a WS client. Example using a small Go program or `wscat`:

- Chat (SSE-bytes): connect `ws://127.0.0.1:8090/v1/chat/completions` with header `Authorization: Bearer <key>`, send `{"model":"<model>","stream":true,"messages":[{"role":"user","content":"hi"}]}`, observe SSE-event text messages arriving, terminating with `[DONE]`.
- Responses (typed events): connect `ws://127.0.0.1:8090/v1/responses`, send `{"type":"response.create","model":"<model>","input":[{"role":"user","content":"hi"}]}`, observe typed events ending with `response.completed`.
- Keepalive: set `StreamingSettings.WebSocketKeepaliveIntervalSeconds` to a small value (e.g. 5) via the admin UI, repeat the chat flow against a slow model, confirm PING frames arrive (visible in Wireshark or a Go client with a PingHandler).

- [ ] **Step 6: Commit**

```bash
git add internal/server/api/openai.go internal/server/api/anthropic.go internal/server/api/gemini.go internal/server/routes.go
git commit -m "feat(api): wire inbound WebSocket routes on streaming endpoints"
```

---

## Task 5: Frontend — Streaming settings card

**Files:**
- Modify: `frontend/src/features/system/data/system.ts`.
- Create: `frontend/src/features/system/components/streaming-settings.tsx`.
- Modify: `frontend/src/features/system/components/tabs.tsx`.
- Modify: `frontend/src/routes/_authenticated/admin/system/index.tsx`.
- Modify: `frontend/src/locales/en/system.json`, `frontend/src/locales/zh-CN/system.json`.

**Interfaces:**
- Consumes: GraphQL `streamingSettings` query / `updateStreamingSettings` mutation (Task 2).
- Produces: `useStreamingSettings`, `useUpdateStreamingSettings` hooks; a mounted "Streaming" tab rendering `StreamingSettings`.

- [ ] **Step 1: Add the data hooks and types**

In `frontend/src/features/system/data/system.ts`, add near the retry-policy block (after `useUpdateRetryPolicy`, ~line 591):

```ts
const STREAMING_SETTINGS_QUERY = `
  query StreamingSettings {
    streamingSettings {
      webSocketKeepaliveIntervalSeconds
    }
  }
`;

const UPDATE_STREAMING_SETTINGS_MUTATION = `
  mutation UpdateStreamingSettings($input: UpdateStreamingSettingsInput!) {
    updateStreamingSettings(input: $input)
  }
`;

export interface StreamingSettings {
  webSocketKeepaliveIntervalSeconds: number;
}

export interface UpdateStreamingSettingsInput {
  webSocketKeepaliveIntervalSeconds?: number;
}

export function useStreamingSettings() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['streamingSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ streamingSettings: StreamingSettings }>(STREAMING_SETTINGS_QUERY);
        return data.streamingSettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateStreamingSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateStreamingSettingsInput) => {
      const data = await graphqlRequest<{ updateStreamingSettings: boolean }>(UPDATE_STREAMING_SETTINGS_MUTATION, { input });
      return data.updateStreamingSettings;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['streamingSettings'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}
```

- [ ] **Step 2: Create the card component**

Create `frontend/src/features/system/components/streaming-settings.tsx`:

```tsx
'use client';

import React, { useCallback, useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useStreamingSettings, useUpdateStreamingSettings, type UpdateStreamingSettingsInput } from '../data/system';

export function StreamingSettings() {
  const { t } = useTranslation();
  const { data: streamingSettings, isLoading } = useStreamingSettings();
  const updateStreamingSettings = useUpdateStreamingSettings();

  const [keepalive, setKeepalive] = useState<number>(0);

  useEffect(() => {
    if (streamingSettings) {
      setKeepalive(streamingSettings.webSocketKeepaliveIntervalSeconds ?? 0);
    }
  }, [streamingSettings]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const input: UpdateStreamingSettingsInput = {
        webSocketKeepaliveIntervalSeconds: Number.isFinite(keepalive) && keepalive > 0 ? Math.floor(keepalive) : 0,
      };
      await updateStreamingSettings.mutateAsync(input);
    },
    [updateStreamingSettings, keepalive]
  );

  if (isLoading) {
    return (
      <div className='flex items-center justify-center p-8'>
        <Loader2 className='h-8 w-8 animate-spin' />
      </div>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.streaming.title')}</CardTitle>
        <CardDescription>{t('system.streaming.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className='space-y-6'>
          <div className='space-y-2'>
            <Label htmlFor='ws-keepalive'>{t('system.streaming.keepalive.label')}</Label>
            <div className='text-muted-foreground mb-2 text-sm'>{t('system.streaming.keepalive.description')}</div>
            <div className='flex items-center space-x-2'>
              <Input
                id='ws-keepalive'
                type='number'
                min='0'
                max='3600'
                value={keepalive}
                onChange={(e) => setKeepalive(Number(e.target.value) || 0)}
                className='w-32'
              />
              <span className='text-muted-foreground text-sm'>s</span>
            </div>
          </div>

          <div className='flex justify-end'>
            <Button type='submit' disabled={updateStreamingSettings.isPending} className='min-w-24'>
              {updateStreamingSettings.isPending ? <Loader2 className='h-4 w-4 animate-spin' /> : t('common.buttons.save')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 3: Register the tab**

In `frontend/src/features/system/components/tabs.tsx`:

- Add `'streaming'` to the `SystemTabKey` union (line 21).
- Add the import after the `RetrySettings` import (line 13):

```tsx
import { StreamingSettings } from './streaming-settings';
```

- Add a `<TabsTrigger>` in the `<TabsList>` next to the `retry` trigger (around line 80-83):

```tsx
<TabsTrigger value='streaming' data-value='streaming'>
  {t('system.tabs.streaming')}
</TabsTrigger>
```

- Add a `<TabsContent>` next to the `retry` content (around line 130-133):

```tsx
<TabsContent value='streaming' className='mt-0 p-0'>
  <StreamingSettings />
</TabsContent>
```

In `frontend/src/routes/_authenticated/admin/system/index.tsx`, add `'streaming'` to the local `SystemTabKey` type (line 5) so the search validator accepts the new tab.

- [ ] **Step 4: Add locale copy**

In `frontend/src/locales/en/system.json`, add (inside the `system.tabs` object and a new `system.streaming` object):

```json
"streaming": "Streaming"
```

and

```json
"streaming": {
  "title": "Streaming",
  "description": "Configure inbound WebSocket streaming behavior.",
  "keepalive": {
    "label": "WebSocket keepalive interval",
    "description": "Send a keepalive frame every N seconds during WebSocket streaming responses to prevent idle timeout. Set to 0 or leave empty to disable."
  }
}
```

In `frontend/src/locales/zh-CN/system.json`, add the matching keys:

```json
"streaming": "流式响应"
```

and

```json
"streaming": {
  "title": "流式响应",
  "description": "配置入站 WebSocket 流式响应行为。",
  "keepalive": {
    "label": "WebSocket 保活间隔",
    "description": "WebSocket 流式响应时每隔 N 秒发送空格以防止空闲超时，设置为 0 或留空表示禁用。"
  }
}
```

(Place these objects at the correct JSON nesting level alongside existing `system.tabs.*` and other `system.*` entries; preserve trailing-comma rules of the existing files.)

- [ ] **Step 5: Verify the frontend**

Run (only if the user explicitly asks; the dev server is managed): `pnpm --filter frontend build` or rely on the running dev server.
Expected: the System Settings page shows a "Streaming" / "流式响应" tab; setting a value and saving persists (reload shows the saved value).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/features/system/data/system.ts frontend/src/features/system/components/streaming-settings.tsx frontend/src/features/system/components/tabs.tsx frontend/src/routes/_authenticated/admin/system/index.tsx frontend/src/locales/en/system.json frontend/src/locales/zh-CN/system.json
git commit -m "feat(frontend): add Streaming settings card for WS keepalive"
```

---

## Rollout

- After all tasks: full backend verification (`go build ./...`, `cd llm && go build ./...`, lint both modules, `go test ./...`, `cd llm && go test ./...`).
- Rebuild `axonhub.exe`, restart the local server, and run the Task 4 Step 6 manual checks (chat WS, Responses WS, keepalive PING).
- When the work is complete and verified, merge `feat/ws-inbound-streaming` back to `unstable` (per the project's branch workflow).
