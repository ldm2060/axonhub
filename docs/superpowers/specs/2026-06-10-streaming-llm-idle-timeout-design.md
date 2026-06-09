# Streaming LLM Idle Timeout Design

## Context

AxonHub currently applies request deadlines with `middleware.WithTimeout(duration)`, implemented as a simple `context.WithTimeout` around the request context. LLM API route groups use `server.Config.LLMRequestTimeout`, which defaults to `600s`. The main HTTP server also sets `WriteTimeout` to `max(RequestTimeout, LLMRequestTimeout)`, so long-running SSE responses can be cut off after roughly the same total duration even when chunks are still flowing.

This design changes streaming LLM requests to use an idle timeout instead of a fixed total duration, while preserving fixed deadlines for non-streaming requests.

## Goals

- Keep reasonable total deadlines for non-streaming requests.
- Let streaming LLM responses continue beyond the current ten-minute total timeout as long as chunks keep arriving.
- Stop stalled streams after a configurable period with no chunks.
- Avoid `http.Server.WriteTimeout` cutting off healthy long SSE responses.
- Preserve existing client-disconnect and upstream-error behavior.

## Non-goals

- Changing provider-specific transformation logic.
- Changing public API response formats except for adding a clearer stream idle-timeout error event when possible.
- Introducing a hard total lifetime limit for streaming requests.

## Recommended approach

Use streaming idle timeout only for requests that are known to be streaming after the request body has been parsed. Non-streaming requests keep using the existing total `LLMRequestTimeout` deadline.

This avoids applying a one-size-fits-all route-group timeout to endpoints that can be either streaming or non-streaming, such as OpenAI-compatible chat completions and Anthropic messages.

## Configuration

Add a server config value:

- `server.llm_stream_idle_timeout`
  - Type: `time.Duration`.
  - Default: `120s`.
  - Meaning: maximum time a streaming LLM response may spend without receiving/writing a chunk.
  - `0` disables stream idle timeout if the codebase's config conventions support zero-duration disablement.

Adjust the main server write timeout behavior:

- Keep `ReadTimeout` for request-read protection.
- Do not set the main `http.Server.WriteTimeout` to `LLMRequestTimeout` by default.
- Prefer `WriteTimeout: 0` for the main API server, or a separate explicit `server.write_timeout` config defaulting to `0`.
- Leave the pprof server behavior unchanged.

## Request timeout model

LLM handlers should make the timeout decision after reading and parsing the request body:

1. If the request is non-streaming:
   - Wrap the handler/orchestrator context with `LLMRequestTimeout`.
   - Keep existing error transformation behavior for context deadline expiry.
2. If the request is streaming:
   - Do not wrap it with the ten-minute total `LLMRequestTimeout` deadline.
   - Keep the original request context so client disconnects still cancel work.
   - Apply `LLMStreamIdleTimeout` while reading and writing stream chunks.

Non-LLM routes, admin routes, auth routes, GraphQL routes, and OpenAPI GraphQL routes continue using the existing `RequestTimeout` middleware.

## Streaming idle timeout behavior

Apply idle timeout close to stream writing, where the code can observe chunk progress. The existing stream writers are `WriteSSEStreamWithErrorFormatter` and `WriteBinaryStream`.

Expected behavior:

1. Set streaming headers as today.
2. Start an idle timer using `LLMStreamIdleTimeout`.
3. Each successful chunk read/write resets the idle timer.
4. If the timer fires before the next chunk:
   - Stop reading the upstream stream.
   - Close the upstream stream through existing `defer Close()` paths.
   - Log that the stream ended because of idle timeout.
   - For SSE, write an `error` event when the connection is still writable, then flush.
   - For binary streams, return JSON only if response headers/body have not started; otherwise log and close.
5. If the request context is cancelled first, treat it as client disconnect or caller cancellation, not idle timeout.
6. If the upstream stream returns an error first, preserve existing upstream error handling.

The implementation must avoid a design where a blocking `stream.Next()` prevents the idle timer from firing. If the stream interface has no context-aware `Next`, wrap reading in a helper or stream adapter so the writer can select between chunk arrival, context cancellation, and idle timeout.

## HTTP server write timeout

`http.Server.WriteTimeout` is a connection-level total write deadline for a request. For SSE and other long streaming responses, it is not equivalent to an idle timeout and can terminate healthy streams. The main API server should therefore not derive `WriteTimeout` from `LLMRequestTimeout`.

Recommended default:

```go
WriteTimeout: 0
```

If a configurable write timeout is desired later, it should be separate from request deadlines and default to disabled for the main server.

## Route impact

The implementation plan should inspect and update the actual handler entry points, but the intended coverage is:

- OpenAI-compatible `/v1/chat/completions`.
- Anthropic-compatible `/anthropic/v1/messages` and `/v1/messages`.
- Gemini generate-content routes that support streaming.
- Audio speech streaming paths when they use the shared SSE or binary stream writers.

Group-level `WithTimeout(LLMRequestTimeout)` should not remain on mixed streaming/non-streaming LLM route groups if it can cancel streaming requests before the idle timeout logic gets a chance to govern them.

## Error handling

- SSE idle timeout: emit an `error` event with a clear message such as `stream idle timeout after 120s`, then flush if possible.
- Binary idle timeout before headers/body: return a JSON error response.
- Binary idle timeout after bytes have been written: log and close the stream; the response format cannot be changed mid-stream.
- Client disconnect: preserve current `ctx.Done()` behavior and log as disconnect/cancellation.
- Upstream errors: preserve existing stream error formatting and status behavior.

## Logging

Stream termination logs should distinguish:

- Normal completion.
- Client disconnect or context cancellation.
- Stream idle timeout.
- Upstream stream error.

Include the timeout duration in idle-timeout logs. Existing tracing and thread context should continue to flow through request context and logging middleware.

## Testing strategy

Add focused unit tests around timeout behavior with short test durations.

Required coverage:

1. Non-streaming LLM request still uses `LLMRequestTimeout` as a total deadline.
2. Streaming request is not governed by the old ten-minute total deadline model.
3. Streaming idle timeout stops a stream that produces no chunk before the idle timeout.
4. Each received chunk resets the idle timer, so a stream with intervals shorter than the idle timeout continues.
5. Main server `WriteTimeout` no longer defaults to `LLMRequestTimeout`.
6. Upstream stream errors and client cancellation remain distinguishable from idle timeout.

## Rollout notes

This change should be implemented with minimal provider-specific logic. Prefer shared helpers for:

- Detecting whether a parsed request is streaming.
- Applying non-streaming LLM deadline.
- Applying streaming idle timeout in SSE and binary stream writers.

After backend changes, follow repository verification rules: build, lint, test, rebuild `axonhub.exe`, restart the local server, and verify affected streaming behavior in the browser or an equivalent local client before committing implementation changes.
