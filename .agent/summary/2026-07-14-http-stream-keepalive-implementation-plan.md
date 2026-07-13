# HTTP Stream Keepalive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a persisted, configurable HTTP stream keepalive interval that transparently keeps compatible text streams active through proxies without changing clients or corrupting binary streams.

**Architecture:** Extend the existing JSON-backed `StreamingSettings` and GraphQL/UI chain. Add a protocol-aware keepalive policy to the API output layer, run streaming `Process()` asynchronously only when keepalive is enabled so the handler can flush before the first upstream event, and use a single stream wait primitive that distinguishes real-event idle timeout from downstream heartbeat ticks.

**Tech Stack:** Go 1.26, Gin, gqlgen, Ent key/value system settings, React 19, TypeScript, TanStack Query, i18next.

## Global Constraints

- Default `httpStreamKeepaliveIntervalSeconds` is `0` (disabled).
- No Ent schema or database migration.
- Keepalive bytes never enter LLM pipeline events, persistence, live preview, usage accounting, or binary streams.
- SSE heartbeat is `: keepalive\n\n`; Gemini JSON heartbeat is `\n`.
- HTTP keepalive does not reset `server.llm_stream_idle_timeout`.
- Preserve existing behavior exactly when interval is `0`.
- Do not run `gqlgen generate`; hand-merge GraphQL generated bindings per project memory.
- Do not commit `.exe` files.

---

### Task 1: Persist and expose the HTTP keepalive setting

**Files:**
- Modify: `internal/server/biz/system.go`
- Test: `internal/server/biz/system_test.go`
- Modify: `internal/server/gql/system.graphql`
- Modify: `internal/server/gql/generated.go`
- Modify: `frontend/src/features/system/data/system.ts`

**Interfaces:**
- Produces: `biz.StreamingSettings.HTTPStreamKeepaliveIntervalSeconds int`
- Produces GraphQL field: `httpStreamKeepaliveIntervalSeconds: Int!`

- [ ] Add failing biz tests for negative normalization, positive preservation, legacy JSON zero default, and two-field round trip.
- [ ] Run `go test ./internal/server/biz -run StreamingSettings -count=1`; expect failures because the field is absent.
- [ ] Add `HTTPStreamKeepaliveIntervalSeconds int` with JSON name `http_stream_keepalive_interval_seconds` and normalize negatives to zero.
- [ ] Update `StreamingSettings` and `UpdateStreamingSettingsInput` in `system.graphql`.
- [ ] Hand-merge the new gqlgen complexity field, input unmarshal, output field marshal/context, and ordered field arrays in `generated.go` following the adjacent WebSocket field pattern.
- [ ] Add the field to the frontend GraphQL query and TypeScript read/write interfaces.
- [ ] Run `go test ./internal/server/biz ./internal/server/gql -count=1`; expect PASS.
- [ ] Commit `feat(settings): add HTTP stream keepalive interval`.

### Task 2: Add protocol-aware stream wait and heartbeat primitives

**Files:**
- Modify: `internal/server/api/stream_timeout.go`
- Test: `internal/server/api/stream_timeout_test.go`

**Interfaces:**
- Produces: keepalive interval in `StreamWriteOptions`.
- Produces: wait result that can report a real event, heartbeat due, stream end/error, request cancellation, or real-event idle timeout without spawning overlapping `Next()` calls.

- [ ] Write failing tests proving a heartbeat is reported before a delayed event, repeated heartbeat waits reuse one in-flight `Next()`, and heartbeat ticks do not postpone idle timeout.
- [ ] Run `go test ./internal/server/api -run 'NextStreamEvent|Keepalive' -count=1`; expect FAIL.
- [ ] Extend `StreamWriteOptions` with `KeepaliveInterval time.Duration`.
- [ ] Implement one per-stream waiter that starts one guarded `Next()` goroutine, tracks a single idle timer from the last real event, emits heartbeat ticks independently, closes the stream on cancellation/idle timeout, and recovers/logs panics in the goroutine.
- [ ] Retain the current no-heartbeat behavior for interval `<= 0`.
- [ ] Run focused tests; expect PASS.
- [ ] Commit `feat(streaming): add HTTP heartbeat scheduler`.

### Task 3: Apply heartbeat to compatible stream writers

**Files:**
- Modify: `internal/server/api/chat.go`
- Modify: `internal/server/api/gemini.go`
- Modify: `internal/server/api/aisdk.go`
- Modify: `internal/server/api/playground.go`
- Test: `internal/server/api/stream_timeout_test.go`
- Add or modify focused writer tests under `internal/server/api/`

**Interfaces:**
- Consumes: `StreamWriteOptions.KeepaliveInterval` and stream waiter from Task 2.
- Produces: SSE `: keepalive\n\n`, Gemini JSON `\n`, compatible AI SDK/Playground whitespace heartbeat; binary writer remains heartbeat-free.

- [ ] Write failing writer tests for SSE comment heartbeat, Gemini JSON validity after whitespace heartbeats, AI SDK framing compatibility, disabled behavior, and binary exclusion.
- [ ] Run focused API tests; expect FAIL.
- [ ] Add common text-stream headers: `Cache-Control: no-cache, no-transform` and `X-Accel-Buffering: no`.
- [ ] Make SSE writer emit `: keepalive\n\n` on heartbeat and Flush.
- [ ] Make Gemini array writer emit `\n` on heartbeat and Flush.
- [ ] Enable whitespace heartbeat for AI SDK and Playground only after their parser/framing tests pass; otherwise leave those writers excluded and document the exclusion in code/tests.
- [ ] Ensure real events reset only heartbeat scheduling while the waiter resets the true idle timer only for real events.
- [ ] Keep `WriteBinaryStreamWithOptions` free of heartbeat bytes.
- [ ] Run focused tests; expect PASS.
- [ ] Commit `feat(streaming): keep compatible HTTP streams alive`.

### Task 4: Cover the pre-stream Process phase

**Files:**
- Modify: `internal/server/api/chat.go`
- Modify: `internal/server/api/stream_timeout.go`
- Test: `internal/server/api/chat_timeout_selection_test.go`
- Test: `internal/server/api/stream_timeout_test.go`

**Interfaces:**
- Consumes: protocol heartbeat writer and persisted setting.
- Produces: asynchronous process result wait used only for streaming requests with a supported keepalive policy and interval > 0.

- [ ] Add failing tests for heartbeat before delayed `Process()` result, preserving HTTP error status before first heartbeat, stream error framing after response commitment, cancellation cleanup, and interval `0` retaining synchronous behavior.
- [ ] Run focused API tests; expect FAIL.
- [ ] Add a `ChatCompletionHandlers` helper that reads `SystemService.StreamingSettings()` once per request and converts the HTTP field to `time.Duration`, returning zero on service/error/nil.
- [ ] Add protocol policy metadata to the selected stream writer so `ChatCompletionWithRequest` knows whether and what it may write before `Process()` returns.
- [ ] For supported streaming requests with interval > 0, execute `Process()` in a panic-guarded goroutine and keep all `c.Writer` operations on the handler goroutine.
- [ ] Before first heartbeat, preserve current HTTP error behavior; after commitment, use the selected writer's stream error format.
- [ ] Cancel/return cleanly when request context ends and ensure buffered result channels prevent sender leaks.
- [ ] Pass the same interval to phase-two writers.
- [ ] Run focused API tests; expect PASS.
- [ ] Commit `feat(streaming): keep pre-stream requests alive`.

### Task 5: Add the system settings control and translations

**Files:**
- Modify: `frontend/src/features/system/components/streaming-settings.tsx`
- Modify: `frontend/src/locales/en/system.json`
- Modify: `frontend/src/locales/zh-CN/system.json`

**Interfaces:**
- Consumes/produces: `httpStreamKeepaliveIntervalSeconds` via existing query/mutation hooks.

- [ ] Add component test coverage if this feature has an existing frontend component test harness; otherwise validate through browser in Task 6.
- [ ] Add `httpKeepalive` state, hydrate it from query data with a `0` fallback, normalize submitted values to non-negative integers, and include the field in mutation input.
- [ ] Add the numeric input immediately below WebSocket keepalive with `min=0`, `max=3600`, seconds suffix, and existing form styling.
- [ ] Update section descriptions so Streaming covers WebSocket and HTTP.
- [ ] Add English and Simplified Chinese label/description keys explaining transparent compatible text-stream heartbeats, binary exclusion, zero-disable behavior, and the 20–30 second Cloudflare recommendation.
- [ ] Commit `feat(frontend): configure HTTP stream keepalive`.

### Task 6: End-to-end verification and final commit

**Files:**
- Verify all changed files.
- Store any verification artifacts under `.agent/summary/` and do not commit transient logs/screenshots unless useful documentation.

- [ ] Run `gofmt -w` on changed Go files and confirm `gofmt -l` prints nothing.
- [ ] Run focused API/biz/gql tests.
- [ ] Run `make test-backend-all` as required by `.agent/rules/go-general.md`.
- [ ] Run the full repository-required verification before final commit: root and `llm` build, lint, and tests.
- [ ] Browser-verify the Streaming settings field is directly below WSS, defaults to 0, saves, and reloads.
- [ ] Exercise a delayed streaming HTTP response and observe heartbeat bytes before the first real event, full final response, and no heartbeat in persisted response chunks.
- [ ] Confirm binary streaming output is byte-identical with keepalive enabled.
- [ ] Run `git status --short`, ensure no `.exe` artifacts, and commit any verification-driven fixes.
