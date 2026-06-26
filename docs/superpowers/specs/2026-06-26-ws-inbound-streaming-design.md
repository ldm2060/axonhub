# WebSocket Inbound Streaming for Model Endpoints Design

## Context

AxonHub's external (client-facing) model endpoints are HTTP only. Streaming responses are delivered over SSE (`text/event-stream`) for OpenAI chat / OpenAI Responses / Anthropic, as JSON-array or `?alt=sse` streaming for Gemini, as a Vercel AI SDK data stream, or as raw bytes for TTS. There is **no inbound WebSocket server**; `websocket.Upgrader`/`Upgrade` appears only in tests.

WebSocket is already used on the **outbound** side, exclusively for the OpenAI Responses API, in `llm/transformer/openai/responses/websocket_executor.go`. That executor dials `wss://` (scheme-swapped from the HTTP URL via `toWebSocketURL`), authenticates on the handshake headers, sends the request as a single typed WS message (`{"type":"response.create", ...}` built by `buildWebSocketCreatePayload`), reads the response as typed JSON events (`response.output_text.delta`, …, terminal `response.completed` / `response.failed` / `response.cancelled` / `response.incomplete` / `error`), and pools/reuses connections across turns keyed by session (`Session_id` + scope + auth + URL + headers) with `previous_response_id` incremental sends.

This design adds **inbound** WebSocket transport for the streaming model endpoints, mirroring the outbound connection model, plus a configurable idle keepalive. The WebSocket handshake is itself an HTTP `GET` Upgrade (inherent to the WS protocol); the design is intentionally **not** a "POST endpoint also accepts a plain GET that returns the same body" — the request and response are WS messages, not HTTP request/response semantics.

## Goals

- Let external clients open a WebSocket (`ws://` / `wss://`) to the existing streaming model endpoints and receive the streaming response over the WS channel.
- Mirror the outbound Responses WS connection model: dial a scheme-swapped URL, authenticate on handshake headers, send the request as a WS message, receive the response as WS messages, support multiple requests per connection.
- Cover all streaming formats: OpenAI Responses (typed events, symmetric with the outbound read side) and OpenAI chat / Anthropic / Gemini (custom framing that reuses each format's existing stream output byte-for-byte).
- Add a configurable idle keepalive so proxies/load balancers with short idle timeouts do not close long-running WS streams.
- Reuse the existing orchestrator/pipeline, inbound transformers, channel selection, and persistence; do not duplicate request processing logic.

## Non-goals

- Supporting WebSocket for non-streaming endpoints (embeddings, audio, images).
- Building a multiplexed (concurrent in-flight) request protocol; requests on a single connection are handled serially.
- Changing the outbound Responses WS executor.
- Changing any HTTP/SSE behavior; the existing POST endpoints remain unchanged.
- Defining a browser-friendly auth channel; authentication is header-based only (the JS `WebSocket` API cannot set custom headers), matching the outbound executor and the existing route middleware.

## Recommended approach

Add a generic inbound WebSocket layer that reuses the existing per-format `ChatCompletionHandlers` (inbound transformer + orchestrator) and emits the response through a new WS writer instead of the gin `ResponseWriter`. The WS handshake is handled on the same endpoint paths as the streaming POSTs, inside the existing `apiGroup` so `WithAPIKeyConfig` / `WithGeminiKeyAuth` / `WithSource` / `WithThread` / `WithTrace` middleware apply unchanged (auth comes from the upgrade request headers).

## Connection model (uniform, mirrors outbound)

1. Client dials `ws(s)://<host>/<endpoint path>` — the same path as the streaming HTTP endpoint, scheme-swapped.
2. Authentication is carried by the handshake headers (`Authorization` / `X-API-Key` / `X-Goog-Api-Key`), enforced by the existing route-group middleware before the upgrade completes.
3. Server upgrades the connection, then reads text messages in a loop and handles them **serially** (one request at a time; the next message is read only after the previous response finishes).
4. Each inbound text message is one request. The connection persists until the client closes it or a read/write error occurs. This naturally supports both Responses multi-turn usage (`previous_response_id`) and single-request-then-close clients.

The synthetic request fed into the existing pipeline is built with `httpclient.ReadHTTPRequest` over a cloned upgrade request whose body is the WS message bytes and whose method/path/query/headers come from the upgrade request. This keeps method/path-dependent behavior (e.g. Gemini `:streamGenerateContent` detection in `requestBodyWantsStream`) identical to HTTP.

## Per-format message framing

| Format (endpoint) | Request message | Response messages |
|---|---|---|
| Responses `/v1/responses`, `/v1/responses/compact` | Request body wrapped as `{"type":"response.create", ...}` (same transform as `buildWebSocketCreatePayload`: set `type`, drop `stream`/`background`). `previous_response_id` is passed through for multi-turn. | Typed event JSON objects: `response.output_text.delta`, …, terminal `response.completed` / `response.failed` / `response.cancelled` / `response.incomplete` / `error` — symmetric with the outbound read side (`streamEventType`, `isTerminalWebSocketEvent`). |
| OpenAI chat `/v1/chat/completions` | Full request JSON body (identical to the POST body, `stream: true`). | One WS text message per SSE event, containing the exact event bytes (`event:` / `data:` produced via `sse.Encode`); terminal `[DONE]` then close. |
| Anthropic `/v1/messages`, `/anthropic/v1/messages` | Full request JSON body. | One WS text message per SSE event (preserves the `event:` type), terminal close. |
| Gemini `/v1beta/models/:model:streamGenerateContent` | Full request JSON body. | One WS text message per SSE event (`?alt=sse` shape), terminal close. |

Non-Responses formats use "SSE event bytes per WS message" so that:
- No new client-side format is introduced — a client concatenates received text messages and parses them as SSE.
- Anthropic's `event:` field is preserved.
- Each format's existing stream output is reused byte-for-byte.

Non-streaming requests (`stream: false`) over WS: send a single WS message containing the full response body (chat / Anthropic / Gemini) or the `response.completed` event (Responses), then close normally.

## Keepalive (idle PING)

While a response stream is in flight, the server runs a timer at the configured interval. When no bytes have been written to the client for that interval, the server sends a WebSocket **PING control frame** (not a text/data message) and resets the timer; every real chunk also resets the timer.

- A PING frame is used for **all** formats because it is invisible to the message stream and therefore never corrupts typed-event JSON (Responses) or SSE parsing (other formats).
- `0` / unset disables keepalive entirely.
- Scope: inbound WS streams only. The existing `server.llm_stream_idle_timeout` (upstream stall detector) continues to apply independently — keepalive keeps the client/proxy connection alive; the idle timeout kills truly dead upstreams.

## Configuration

Add a new admin-configurable system setting group so it can grow later:

```go
type StreamingSettings struct {
    WebSocketKeepaliveIntervalSeconds int `json:"web_socket_keepalive_interval_seconds"` // 0 = disabled
}
```

- Stored in the `systems` table under key `streaming_settings`, using the same `getSystemValue` / `setSystemValue` + cache pattern as `retry_policy`.
- `biz.SystemService`: `StreamingSettings(ctx)`, `SetStreamingSettings(ctx, *StreamingSettings)`, and a `normalizeStreamingSettings` that clamps the value to `>= 0` and defaults to `0`.
- GraphQL: `streamingSettings` query and `updateStreamingSettings(input: UpdateStreamingSettingsInput!)` mutation; bind `StreamingSettings` / `UpdateStreamingSettingsInput` to the `biz` type in `internal/server/gql/gqlgen.yml`, then regenerate.
- Frontend: a new "Streaming" card in the System Settings page with a number input, labeled (zh): "WebSocket 流式响应时每隔 N 秒发送空格以防止空闲超时，设置为 0 或留空表示禁用。" Wired with `useStreamingSettings` / `useUpdateStreamingSettings` hooks mirroring the retry-policy pattern.

Note: the user-facing label keeps the original "send a space" wording; the implementation uses PING control frames (see Keepalive). The label describes the user-visible effect (preventing idle timeout), and the help text can clarify the mechanism.

## Routing

Register WebSocket upgrade handlers on the same paths as the streaming POSTs, inside the existing `apiGroup`:

- `GET /v1/responses`
- `GET /v1/responses/compact`
- `GET /v1/chat/completions`
- `GET /v1/messages`
- `GET /anthropic/v1/messages`
- `GET /v1beta/models/:model:streamGenerateContent`

Gin supports POST (HTTP, existing) and GET (WS upgrade) on the same path. The GET handler performs the WS upgrade; the existing POST handlers are untouched.

## Error handling and connection lifecycle

- Auth/upgrade failure: the route-group middleware rejects with HTTP 401 before the upgrade completes (unchanged behavior).
- Request body parse error: send one WS message with the format-appropriate error (Responses: `{"type":"error", ...}`; others: the same error JSON / SSE `error` event bytes the HTTP path produces via the inbound transformer's `TransformError` / `FormatStreamError`), then close normally (1000).
- Mid-stream upstream error: send a final WS message carrying the format-appropriate error frame, then close.
- Client disconnect: detected via WS read/write error or request context cancellation; stop the upstream stream through the existing `defer Close()` paths.
- After the final message (done / error / non-streaming body) the server closes the WS normally.
- All manually started goroutines (keepalive ticker, upstream stream drain) install a top-level `defer recover()` guard that logs the panic before exit, per the Go rules.

## TLS and origins

- `ws://` on a plain listener, `wss://` when the server is configured for TLS. Behind a reverse proxy the proxy terminates TLS and forwards `ws://` to the gateway (standard); no special handling beyond the upgrader.
- The upgrader's `CheckOrigin` respects the server CORS allowed-origins config and otherwise allows API usage, consistent with the existing permissive `Access-Control-Allow-Origin: *` on streams.

## Logging

Stream/connection logs should distinguish:

- Normal completion (terminal event received / stream closed cleanly).
- Client disconnect or context cancellation.
- Keepalive PING activity (debug level) and keepalive disabled.
- Upstream stream error.
- Invalid request message / parse error.

Existing tracing and thread context (`AH-Thread-Id`, `AH-Trace-Id`) continue to flow through the request context and logging middleware, since the WS handler runs inside the same `apiGroup`.

## Testing strategy

Add focused unit tests with a local WS test server and deterministic fake upstreams.

Required coverage:

1. Responses WS: after sending a `response.create` message, the received message sequence equals the expected typed events (symmetric with the outbound read side).
2. Non-Responses WS: each received WS text message equals the exact bytes the HTTP `WriteSSEStream` path produces for one event (byte-level diff), and the terminal `[DONE]` / close is emitted.
3. Non-streaming request over WS returns a single response message then closes.
4. Multiple requests on one connection are handled serially (e.g. two consecutive `response.create` messages each produce a complete, correctly-terminated response).
5. Keepalive: with a slow upstream and a short interval, a PING is sent during idle; with interval `0`, no PING is sent.
6. Auth failure on the upgrade is rejected with 401 before upgrade.
7. Upstream mid-stream error produces the correct final error frame and a clean close.
8. `StreamingSettings` round-trips through `SetStreamingSettings` / `StreamingSettings` and normalizes invalid values to defaults (mirrors the existing `TestClientRestrictionLevel_*` tests).

## Files (outline)

Backend:

- `internal/server/api/ws.go` (new): WS upgrader, serial read loop, synthetic request builder, WS writers (Responses typed-event writer + SSE-event-bytes writer), keepalive PING ticker.
- `internal/server/api/chat.go`: factor out single-SSE-event encoding (use `sse.Encode`) so the WS writer reuses it; keep HTTP paths unchanged.
- `internal/server/routes.go`: register `GET` WS upgrade handlers per streaming endpoint inside `apiGroup`.
- `internal/server/biz/system.go`: `StreamingSettings` get/set/normalize, key constant, default.
- `internal/server/gql/system.graphql`, `internal/server/gql/gqlgen.yml`, regenerated `internal/server/gql/generated.go`, `internal/server/gql/system.resolvers.go`: `streamingSettings` query / `updateStreamingSettings` mutation.
- Reuse `llm/transformer/openai/responses` event codec helpers where applicable for the Responses typed-event writer.

Frontend:

- `frontend/src/features/system/data/system.ts`: `STREAMING_SETTINGS_QUERY` / `UPDATE_STREAMING_SETTINGS_MUTATION`, `StreamingSettings` / input types, `useStreamingSettings` / `useUpdateStreamingSettings` hooks.
- `frontend/src/features/system/components/streaming-settings.tsx` (new): the Streaming card; mount in the system settings page.
- `frontend/src/locales/en/system.json`, `frontend/src/locales/zh-CN/system.json`: label / description copy.

## Rollout notes

- Keep the WS layer thin and reuse the existing orchestrator; do not fork request processing.
- After backend changes, follow repository verification rules: `go build ./...`, `cd llm && go build ./...`, lint, `make test-backend-all` / `go test ./...`, rebuild `axonhub.exe`, restart the local server, and verify the WS endpoints with a local WS client before committing.
- The WebSocket dependency (`github.com/gorilla/websocket`) is already present (used by the outbound executor), so no new module is required.
