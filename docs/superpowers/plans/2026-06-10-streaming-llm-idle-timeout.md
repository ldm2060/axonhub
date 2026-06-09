# Streaming LLM Idle Timeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve total request deadlines for non-streaming LLM requests while allowing streaming LLM responses to run past the old ten-minute cap as long as chunks keep arriving, with a configurable idle timeout for stalled streams.

**Architecture:** Move LLM timeout selection from mixed route-group middleware into the parsed-request handler path. Add a focused stream writer options/helper layer that enforces idle timeout by running blocking `stream.Next()` calls in a goroutine and selecting between chunk arrival, request cancellation, and idle timer expiry. Disable the main `http.Server.WriteTimeout` by default so healthy SSE streams are not cut by a connection-level total write deadline.

**Tech Stack:** Go 1.26, Gin, `net/http`, existing `llm/streams.Stream[*httpclient.StreamEvent]`, existing AxonHub config via `conf` tags and Viper defaults.

---

## File structure

**Create:**
- `internal/server/api/stream_timeout.go` — stream idle-timeout helpers, stream writer options, and timeout error type. This keeps timing/concurrency logic out of provider-specific handlers.
- `internal/server/api/stream_timeout_test.go` — unit tests for idle timeout, chunk reset, upstream errors, and cancellation using fake streams.

**Modify:**
- `internal/server/config.go` — add `LLMStreamIdleTimeout time.Duration`.
- `conf/conf.go` — default `server.llm_stream_idle_timeout` to `120s`.
- `internal/server/server.go` — set main server `WriteTimeout` to `0` instead of deriving it from LLM request timeout.
- `internal/server/routes.go` — remove `middleware.WithTimeout(server.Config.LLMRequestTimeout)` from mixed LLM route groups; keep non-LLM `RequestTimeout` middleware. Also remove it from `/admin/playground/chat` because that handler can stream.
- `internal/server/api/chat.go` — add `RequestTimeout`/`StreamIdleTimeout` fields to `ChatCompletionHandlers`, choose per parsed request whether to wrap context with total timeout, and pass stream options to writers.
- `internal/server/api/aisdk.go` — update `WriteJSONStream` to use shared stream idle-timeout helper.
- `internal/server/api/gemini.go` — update `WriteGeminiStream` to use shared stream idle-timeout helper.
- `internal/server/api/openai.go`, `internal/server/api/anthropic.go`, `internal/server/api/gemini.go`, `internal/server/api/aisdk.go`, `internal/server/api/playground.go` — pass timeout config into `ChatCompletionHandlers` created in constructors.
- `internal/server/api/chat_test.go` — add handler-level tests for streaming vs non-streaming timeout context selection.
- `internal/server/server_test.go` — add a focused test for default main server write-timeout behavior if no suitable test exists.

**Do not modify:**
- Provider transformers in `llm/transformer/...` unless tests reveal that a request's `Stream` field is not populated after inbound transformation.
- `llm/streams.Stream` interface; keep the idle-timeout wrapper local to server API code to avoid broad module changes.

---

### Task 1: Add stream idle-timeout config and disable server write timeout

**Files:**
- Modify: `internal/server/config.go`
- Modify: `conf/conf.go`
- Modify: `internal/server/server.go`
- Test: `internal/server/server_test.go`

- [ ] **Step 1: Write the failing server config/write-timeout tests**

Create `internal/server/server_test.go` if it does not exist. Add this test content:

```go
package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPServerDisablesWriteTimeoutForLongStreams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	srv := &Server{
		Config: Config{
			Host:                    "127.0.0.1",
			Port:                    8090,
			ReadTimeout:             5 * time.Second,
			RequestTimeout:          30 * time.Second,
			LLMRequestTimeout:       10 * time.Minute,
			LLMStreamIdleTimeout:    2 * time.Minute,
		},
		Engine: gin.New(),
	}

	httpServer := srv.newHTTPServer("127.0.0.1:8090")

	require.Equal(t, 5*time.Second, httpServer.ReadTimeout)
	require.Zero(t, httpServer.WriteTimeout)
	require.IsType(t, &http.Server{}, httpServer)
}
```

This test assumes Task 1 will extract server construction to `newHTTPServer` so it can be tested without opening a socket.

- [ ] **Step 2: Run the test to verify it fails**

Run:

```powershell
go test ./internal/server -run TestNewHTTPServerDisablesWriteTimeoutForLongStreams -count=1
```

Expected: FAIL with `srv.newHTTPServer undefined` and/or `LLMStreamIdleTimeout undefined`.

- [ ] **Step 3: Add the config field**

In `internal/server/config.go`, update `Config`:

```go
	// LLMRequestTimeout is the maximum duration for processing a non-streaming request to LLM.
	LLMRequestTimeout time.Duration `conf:"llm_request_timeout" yaml:"llm_request_timeout" json:"llm_request_timeout"`

	// LLMStreamIdleTimeout is the maximum duration a streaming LLM request may wait without chunks.
	LLMStreamIdleTimeout time.Duration `conf:"llm_stream_idle_timeout" yaml:"llm_stream_idle_timeout" json:"llm_stream_idle_timeout"`
```

Keep the existing `LLMRequestTimeout` field in place; only update its comment and add the new field immediately after it.

- [ ] **Step 4: Add the default value**

In `conf/conf.go`, update `setDefaults` near the existing LLM timeout default:

```go
	v.SetDefault("server.request_timeout", "30s")
	v.SetDefault("server.llm_request_timeout", "600s")
	v.SetDefault("server.llm_stream_idle_timeout", "120s")
```

- [ ] **Step 5: Extract HTTP server construction and disable WriteTimeout**

In `internal/server/server.go`, add this method near `Run()`:

```go
func (srv *Server) newHTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      srv.Engine,
		ReadTimeout:  srv.Config.ReadTimeout,
		WriteTimeout: 0,
	}
}
```

Then replace the inline server construction in `Run()`:

```go
	srv.server = srv.newHTTPServer(addr)
```

- [ ] **Step 6: Run the focused test to verify it passes**

Run:

```powershell
go test ./internal/server -run TestNewHTTPServerDisablesWriteTimeoutForLongStreams -count=1
```

Expected: PASS.

- [ ] **Step 7: Run root formatting/build smoke**

Run:

```powershell
gofmt -w "internal/server/config.go" "internal/server/server.go" "internal/server/server_test.go" "conf/conf.go"
go test ./internal/server ./conf -count=1
go build ./...
```

Expected: all commands exit 0.

- [ ] **Step 8: Commit Task 1**

Run:

```powershell
git add "internal/server/config.go" "conf/conf.go" "internal/server/server.go" "internal/server/server_test.go"
git commit -m @'
feat(server): configure streaming idle timeout

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
'@
```

---

### Task 2: Add stream idle-timeout helper and tests

**Files:**
- Create: `internal/server/api/stream_timeout.go`
- Create: `internal/server/api/stream_timeout_test.go`

- [ ] **Step 1: Write failing tests for stream polling behavior**

Create `internal/server/api/stream_timeout_test.go` with these tests:

```go
package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm/httpclient"
)

type blockingTestStream struct {
	nextCh  chan *httpclient.StreamEvent
	err     error
	closed  bool
}

func newBlockingTestStream() *blockingTestStream {
	return &blockingTestStream{nextCh: make(chan *httpclient.StreamEvent)}
}

func (s *blockingTestStream) Next() bool {
	event, ok := <-s.nextCh
	if !ok {
		return false
	}

	s.current = event
	return true
}

func (s *blockingTestStream) Current() *httpclient.StreamEvent { return s.current }
func (s *blockingTestStream) Err() error                       { return s.err }
func (s *blockingTestStream) Close() error {
	s.closed = true
	return nil
}

func TestNextStreamEventTimesOutWhenNoChunkArrives(t *testing.T) {
	stream := newBlockingTestStream()
	ctx := context.Background()

	result := nextStreamEvent(ctx, stream, 10*time.Millisecond)

	require.False(t, result.ok)
	require.ErrorIs(t, result.err, ErrStreamIdleTimeout)
}

func TestNextStreamEventReturnsChunkBeforeTimeout(t *testing.T) {
	stream := newBlockingTestStream()
	ctx := context.Background()
	want := &httpclient.StreamEvent{Type: "message", Data: []byte(`{"ok":true}`)}

	go func() {
		stream.nextCh <- want
	}()

	result := nextStreamEvent(ctx, stream, time.Second)

	require.True(t, result.ok)
	require.NoError(t, result.err)
	require.Equal(t, want, result.event)
}

func TestNextStreamEventReturnsContextCancellation(t *testing.T) {
	stream := newBlockingTestStream()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := nextStreamEvent(ctx, stream, time.Second)

	require.False(t, result.ok)
	require.ErrorIs(t, result.err, context.Canceled)
}

func TestNextStreamEventReturnsUpstreamErrorAtEOF(t *testing.T) {
	stream := newBlockingTestStream()
	stream.err = errors.New("upstream failed")
	close(stream.nextCh)

	result := nextStreamEvent(context.Background(), stream, time.Second)

	require.False(t, result.ok)
	require.EqualError(t, result.err, "upstream failed")
}
```

Important: the test stream needs a `current *httpclient.StreamEvent` field. Add it to `blockingTestStream` in the test code above:

```go
	current *httpclient.StreamEvent
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/server/api -run 'TestNextStreamEvent' -count=1
```

Expected: FAIL with `nextStreamEvent undefined` and `ErrStreamIdleTimeout undefined`.

- [ ] **Step 3: Implement stream timeout helper**

Create `internal/server/api/stream_timeout.go`:

```go
package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/streams"
)

var ErrStreamIdleTimeout = errors.New("stream idle timeout")

type StreamWriteOptions struct {
	IdleTimeout time.Duration
}

type streamNextResult struct {
	event *httpclient.StreamEvent
	ok    bool
	err   error
}

func nextStreamEvent(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent], idleTimeout time.Duration) streamNextResult {
	if idleTimeout <= 0 {
		if stream.Next() {
			return streamNextResult{event: stream.Current(), ok: true}
		}

		return streamNextResult{err: stream.Err()}
	}

	resultCh := make(chan streamNextResult, 1)
	go func() {
		if stream.Next() {
			resultCh <- streamNextResult{event: stream.Current(), ok: true}
			return
		}

		resultCh <- streamNextResult{err: stream.Err()}
	}()

	timer := time.NewTimer(idleTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return streamNextResult{err: ctx.Err()}
	case <-timer.C:
		return streamNextResult{err: fmt.Errorf("%w after %s", ErrStreamIdleTimeout, idleTimeout)}
	case result := <-resultCh:
		return result
	}
}
```

- [ ] **Step 4: Run helper tests to verify they pass**

Run:

```powershell
go test ./internal/server/api -run 'TestNextStreamEvent' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

Run:

```powershell
git add "internal/server/api/stream_timeout.go" "internal/server/api/stream_timeout_test.go"
git commit -m @'
feat(api): add stream idle timeout helper

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
'@
```

---

### Task 3: Apply idle timeout to SSE, AI SDK, Gemini JSON, and binary stream writers

**Files:**
- Modify: `internal/server/api/chat.go`
- Modify: `internal/server/api/aisdk.go`
- Modify: `internal/server/api/gemini.go`
- Test: `internal/server/api/stream_timeout_test.go`
- Test: `internal/server/api/chat_test.go`

- [ ] **Step 1: Add failing writer tests for SSE idle timeout and chunk reset**

Append to `internal/server/api/stream_timeout_test.go`:

```go
func TestWriteSSEStreamWithOptionsEmitsErrorOnIdleTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	stream := newBlockingTestStream()

	WriteSSEStreamWithOptions(c, stream, StreamWriteOptions{IdleTimeout: 10 * time.Millisecond})

	body := w.Body.String()
	require.Contains(t, body, "event:error")
	require.Contains(t, body, "stream idle timeout")
}

func TestWriteSSEStreamWithOptionsResetsIdleTimeoutAfterChunk(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	stream := newBlockingTestStream()

	go func() {
		stream.nextCh <- &httpclient.StreamEvent{Data: []byte(`{"step":1}`)}
		time.Sleep(5 * time.Millisecond)
		stream.nextCh <- &httpclient.StreamEvent{Data: []byte(`{"step":2}`)}
		close(stream.nextCh)
	}()

	WriteSSEStreamWithOptions(c, stream, StreamWriteOptions{IdleTimeout: 50 * time.Millisecond})

	body := w.Body.String()
	require.Contains(t, body, `{"step":1}`)
	require.Contains(t, body, `{"step":2}`)
	require.NotContains(t, body, "stream idle timeout")
}
```

Add imports needed by the appended tests:

```go
import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/server/api -run 'TestWriteSSEStreamWithOptions' -count=1
```

Expected: FAIL with `WriteSSEStreamWithOptions undefined`.

- [ ] **Step 3: Add option-aware SSE writer and preserve existing public wrapper**

In `internal/server/api/chat.go`, keep `WriteSSEStream` and `WriteSSEStreamWithErrorFormatter` as compatibility wrappers, then add `WriteSSEStreamWithOptions` and `WriteSSEStreamWithOptionsAndErrorFormatter`:

```go
func WriteSSEStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	WriteSSEStreamWithOptionsAndErrorFormatter(c, stream, StreamWriteOptions{}, FormatStreamError)
}

func WriteSSEStreamWithOptions(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions) {
	WriteSSEStreamWithOptionsAndErrorFormatter(c, stream, opts, FormatStreamError)
}

func WriteSSEStreamWithErrorFormatter(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], formatErr StreamErrorFormatter) {
	WriteSSEStreamWithOptionsAndErrorFormatter(c, stream, StreamWriteOptions{}, formatErr)
}

func WriteSSEStreamWithOptionsAndErrorFormatter(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions, formatErr StreamErrorFormatter) {
	ctx := c.Request.Context()
	clientDisconnected := false

	if formatErr == nil {
		formatErr = FormatStreamError
	}

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	c.Header("Content-Type", sse.ContentType)
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()

	for {
		result := nextStreamEvent(ctx, stream, opts.IdleTimeout)
		if result.ok {
			cur := result.event
			c.SSEvent(cur.Type, cur.Data)
			log.Debug(ctx, "write stream event", log.Any("event", cur))
			c.Writer.Flush()
			continue
		}

		if result.err != nil {
			if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
				clientDisconnected = true
				log.Warn(ctx, "Context done, stopping stream", log.Cause(result.err))
				return
			}

			if errors.Is(result.err, ErrStreamIdleTimeout) {
				log.Warn(ctx, "Stream idle timeout, stopping stream", log.Duration("idle_timeout", opts.IdleTimeout), log.Cause(result.err))
			} else {
				log.Error(ctx, "Error in stream", log.Cause(result.err))
			}

			c.SSEvent("error", formatErr(ctx, result.err))
		}

		c.Writer.Flush()
		return
	}
}
```

- [ ] **Step 4: Update `WriteBinaryStream` to use `nextStreamEvent`**

In `internal/server/api/chat.go`, add an option-aware binary writer:

```go
func WriteBinaryStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	WriteBinaryStreamWithOptions(c, stream, StreamWriteOptions{})
}

func WriteBinaryStreamWithOptions(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions) {
	// Keep the existing body, but replace the `if !stream.Next()` branch with:
	result := nextStreamEvent(ctx, stream, opts.IdleTimeout)
	if !result.ok {
		if result.err != nil {
			if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
				clientDisconnected = true
				log.Warn(ctx, "Context done, stopping binary stream", log.Cause(result.err))
				return
			}

			log.Error(ctx, "Error in binary stream", log.Cause(result.err))
			if !headersWritten {
				c.JSON(streamErrorStatus(result.err), FormatStreamError(ctx, result.err))
				return
			}
		}

		c.Writer.Flush()
		return
	}

	cur := result.event
	// Continue with the existing BinaryStreamDoneEventType and write logic.
}
```

Do not duplicate both full binary writer bodies. Convert the existing `WriteBinaryStream` body in place into `WriteBinaryStreamWithOptions`, then make `WriteBinaryStream` call it.

- [ ] **Step 5: Update AI SDK data stream writer**

In `internal/server/api/aisdk.go`, preserve the old wrapper and add an option-aware function:

```go
func WriteJSONStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	WriteJSONStreamWithOptions(c, stream, StreamWriteOptions{})
}

func WriteJSONStreamWithOptions(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions) {
	// Keep existing headers. In the loop, use nextStreamEvent(ctx, stream, opts.IdleTimeout).
	// On ErrStreamIdleTimeout write: 3:"stream idle timeout after <duration>"\n
	if errors.Is(result.err, ErrStreamIdleTimeout) {
		log.Warn(ctx, "Stream idle timeout, stopping AI SDK stream", log.Duration("idle_timeout", opts.IdleTimeout), log.Cause(result.err))
		_, _ = c.Writer.Write([]byte("3:" + `"` + result.err.Error() + `"` + "\n"))
		return
	}
}
```

When writing the final code, avoid leaving pseudocode. The full function must compile and mirror the existing `WriteJSONStream` behavior.

- [ ] **Step 6: Update Gemini JSON stream writer**

In `internal/server/api/gemini.go`, preserve the old wrapper and add an option-aware function:

```go
func WriteGeminiStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	WriteGeminiStreamWithOptions(c, stream, StreamWriteOptions{})
}

func WriteGeminiStreamWithOptions(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions) {
	// Keep existing JSON-array response format.
	// On idle timeout, log and close the array with `]` so the response remains syntactically valid if possible.
}
```

When writing the final code, include complete compiled logic. Do not emit the comments above as the implementation.

- [ ] **Step 7: Run writer tests**

Run:

```powershell
gofmt -w "internal/server/api/chat.go" "internal/server/api/aisdk.go" "internal/server/api/gemini.go" "internal/server/api/stream_timeout_test.go"
go test ./internal/server/api -run 'TestNextStreamEvent|TestWriteSSEStream|TestWriteSSEStreamWithOptions' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit Task 3**

Run:

```powershell
git add "internal/server/api/chat.go" "internal/server/api/aisdk.go" "internal/server/api/gemini.go" "internal/server/api/stream_timeout_test.go"
git commit -m @'
feat(api): enforce idle timeout for streaming responses

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
'@
```

---

### Task 4: Choose total deadline only after parsing LLM request body

**Files:**
- Modify: `internal/server/api/chat.go`
- Modify: `internal/server/api/openai.go`
- Modify: `internal/server/api/anthropic.go`
- Modify: `internal/server/api/gemini.go`
- Modify: `internal/server/api/aisdk.go`
- Modify: `internal/server/api/playground.go`
- Test: `internal/server/api/chat_test.go`

- [ ] **Step 1: Write a failing test for stream detection and context deadline selection**

Append to `internal/server/api/chat_test.go`:

```go
type timeoutSelectionOrchestrator struct {
	lastDeadlineSet bool
	lastDeadline    time.Time
	result          orchestrator.ChatCompletionResult
}

func (o *timeoutSelectionOrchestrator) Process(ctx context.Context, _ *httpclient.Request) (*orchestrator.ChatCompletionResult, error) {
	deadline, ok := ctx.Deadline()
	o.lastDeadlineSet = ok
	o.lastDeadline = deadline
	return &o.result, nil
}

func TestChatCompletionWithRequestAppliesTotalDeadlineToNonStreamingRequest(t *testing.T) {
	// If ChatCompletionOrchestrator is concrete and cannot be replaced, do not use this exact fake.
	// Instead, introduce a small interface in chat.go as described in Step 3 and use it here.
}
```

Because `ChatCompletionHandlers.ChatCompletionOrchestrator` is currently a concrete `*orchestrator.ChatCompletionOrchestrator`, this test will not compile until Step 3 introduces a small interface. The final test should construct a handler with:

```go
handler := &ChatCompletionHandlers{
	Processor:            fake,
	RequestTimeout:       20 * time.Millisecond,
	StreamIdleTimeout:    time.Second,
	StreamWriter: func(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions) {
		// no-op for this test
	},
}
```

Use two request bodies:

```go
nonStreamingReq := &httpclient.Request{Body: []byte(`{"model":"test","stream":false,"messages":[{"role":"user","content":"hi"}]}`)}
streamingReq := &httpclient.Request{Body: []byte(`{"model":"test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)}
```

Assertions:

```go
require.True(t, fake.lastDeadlineSet, "non-streaming request should get total deadline")
require.False(t, fake.lastDeadlineSet, "streaming request should not get total LLMRequestTimeout deadline")
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run:

```powershell
go test ./internal/server/api -run 'TestChatCompletionWithRequestAppliesTotalDeadline|TestChatCompletionWithRequestDoesNotApplyTotalDeadlineToStreamingRequest' -count=1
```

Expected: FAIL because the handler is not yet interface-driven / timeout-aware.

- [ ] **Step 3: Introduce small processor and stream writer interfaces**

In `internal/server/api/chat.go`, replace the current stream writer type with option-aware form and introduce processor interface:

```go
type ChatCompletionProcessor interface {
	Process(ctx context.Context, genericReq *httpclient.Request) (*orchestrator.ChatCompletionResult, error)
}

type StreamWriter func(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions)

type ChatCompletionHandlers struct {
	ChatCompletionOrchestrator *orchestrator.ChatCompletionOrchestrator
	Processor                  ChatCompletionProcessor
	StreamWriter               StreamWriter
	RequestTimeout             time.Duration
	StreamIdleTimeout          time.Duration
}
```

Add helper:

```go
func (handlers *ChatCompletionHandlers) processor() ChatCompletionProcessor {
	if handlers.Processor != nil {
		return handlers.Processor
	}

	return handlers.ChatCompletionOrchestrator
}
```

Update `NewChatCompletionHandlers`:

```go
func NewChatCompletionHandlers(orchestrator *orchestrator.ChatCompletionOrchestrator) *ChatCompletionHandlers {
	return &ChatCompletionHandlers{
		ChatCompletionOrchestrator: orchestrator,
		Processor:                  orchestrator,
		StreamWriter:               WriteSSEStreamWithOptions,
	}
}
```

Update `WithStreamWriter` to preserve timeout fields:

```go
func (handlers *ChatCompletionHandlers) WithStreamWriter(writer StreamWriter) *ChatCompletionHandlers {
	return &ChatCompletionHandlers{
		ChatCompletionOrchestrator: handlers.ChatCompletionOrchestrator,
		Processor:                  handlers.processor(),
		StreamWriter:               writer,
		RequestTimeout:             handlers.RequestTimeout,
		StreamIdleTimeout:          handlers.StreamIdleTimeout,
	}
}
```

- [ ] **Step 4: Add parsed stream detection helper**

In `internal/server/api/chat.go`, add:

```go
func requestBodyWantsStream(genericReq *httpclient.Request) bool {
	if genericReq == nil || len(genericReq.Body) == 0 {
		return false
	}

	return gjson.GetBytes(genericReq.Body, "stream").Bool()
}
```

This deliberately checks the inbound body before transformation. It covers OpenAI-compatible, Anthropic-compatible, Gemini-compatible, and AI SDK JSON requests that carry a boolean `stream` flag. If a later provider uses a different marker, add a provider-specific helper in that handler instead of changing this generic boolean behavior.

- [ ] **Step 5: Wrap only non-streaming requests with total timeout**

In `ChatCompletionWithRequest`, replace:

```go
ctx := c.Request.Context()
...
result, err := handlers.ChatCompletionOrchestrator.Process(ctx, genericReq)
```

with:

```go
ctx := c.Request.Context()
streaming := requestBodyWantsStream(genericReq)
if !streaming && handlers.RequestTimeout > 0 {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, handlers.RequestTimeout)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
}

result, err := handlers.processor().Process(ctx, genericReq)
```

When writing stream response, pass idle timeout options:

```go
streamWriter := handlers.StreamWriter
if streamWriter == nil {
	streamWriter = WriteSSEStreamWithOptions
}

streamWriter(c, newUpstreamErrorStream(ctx, result.ChatCompletionStream, handlers.ChatCompletionOrchestrator.SystemService), StreamWriteOptions{IdleTimeout: handlers.StreamIdleTimeout})
```

For tests using only `Processor`, guard direct `handlers.ChatCompletionOrchestrator.SystemService` access with a helper:

```go
func (handlers *ChatCompletionHandlers) systemService() *biz.SystemService {
	if handlers.ChatCompletionOrchestrator == nil {
		return nil
	}
	return handlers.ChatCompletionOrchestrator.SystemService
}
```

Then call `newUpstreamErrorStream(ctx, result.ChatCompletionStream, handlers.systemService())`.

- [ ] **Step 6: Pass timeouts from constructors**

Add `ServerConfig server.Config` to API handler params only if the existing dependency graph can provide it without cycles. The simplest pattern is to add `Config server.Config` to each relevant `fx.In` params struct using an import alias:

```go
serverconfig "github.com/ldm2060/axonhub/internal/server"
```

Do not import package `server` into `internal/server/api` if it creates an import cycle. If it creates a cycle, create a tiny local config provider in `internal/server/api`:

```go
type TimeoutConfig struct {
	LLMRequestTimeout    time.Duration
	LLMStreamIdleTimeout time.Duration
}
```

and provide it from `internal/server/server.go` or `internal/server/dependencies` where `server.Config` is already available.

Set every `ChatCompletionHandlers` created in constructors:

```go
handler := &ChatCompletionHandlers{
	ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(...),
	RequestTimeout:             params.Config.LLMRequestTimeout,
	StreamIdleTimeout:          params.Config.LLMStreamIdleTimeout,
	StreamWriter:               WriteSSEStreamWithOptions,
}
```

For AI SDK:

```go
StreamWriter: WriteJSONStreamWithOptions,
```

For Gemini alt handlers:

```go
handlers.ChatCompletionHandlers.WithStreamWriter(WriteSSEStreamWithOptions).ChatCompletion(c)
handlers.ChatCompletionHandlers.WithStreamWriter(WriteGeminiStreamWithOptions).ChatCompletion(c)
```

- [ ] **Step 7: Run focused tests**

Run:

```powershell
gofmt -w "internal/server/api/chat.go" "internal/server/api/openai.go" "internal/server/api/anthropic.go" "internal/server/api/gemini.go" "internal/server/api/aisdk.go" "internal/server/api/playground.go" "internal/server/api/chat_test.go"
go test ./internal/server/api -run 'TestChatCompletionWithRequest.*Deadline|TestWriteSSEStreamWithOptions|TestNextStreamEvent' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit Task 4**

Run:

```powershell
git add "internal/server/api/chat.go" "internal/server/api/openai.go" "internal/server/api/anthropic.go" "internal/server/api/gemini.go" "internal/server/api/aisdk.go" "internal/server/api/playground.go" "internal/server/api/chat_test.go"
git commit -m @'
feat(api): select llm timeout after parsing stream flag

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
'@
```

---

### Task 5: Remove mixed LLM route-group timeout middleware

**Files:**
- Modify: `internal/server/routes.go`
- Test: `internal/server/routes_test.go` if route tests already exist; otherwise rely on handler tests and full suite.

- [ ] **Step 1: Inspect routes that still use `LLMRequestTimeout` middleware**

Run:

```powershell
Select-String -Path "internal/server/routes.go" -Pattern "LLMRequestTimeout"
```

Expected before changes: entries for `/admin/playground/chat`, root LLM `apiGroup`, Gemini group, and Gemini alias group.

- [ ] **Step 2: Remove LLM timeout middleware from mixed streaming route groups**

In `internal/server/routes.go`, change `/admin/playground/chat` from:

```go
adminGroup.POST(
	"/playground/chat",
	middleware.WithTimeout(server.Config.LLMRequestTimeout),
	middleware.WithSource(request.SourcePlayground),
	handlers.Playground.ChatCompletion,
)
```

to:

```go
adminGroup.POST(
	"/playground/chat",
	middleware.WithSource(request.SourcePlayground),
	handlers.Playground.ChatCompletion,
)
```

Change root LLM `apiGroup` from:

```go
apiGroup := server.Group("/",
	middleware.WithTimeout(server.Config.LLMRequestTimeout),
	middleware.WithIPBlocklist(services.SystemService),
```

to:

```go
apiGroup := server.Group("/",
	middleware.WithIPBlocklist(services.SystemService),
```

Change Gemini groups similarly by removing only this middleware:

```go
middleware.WithTimeout(server.Config.LLMRequestTimeout),
```

Do not remove `RequestTimeout` middleware from public/admin/auth/OpenAPI GraphQL routes.

- [ ] **Step 3: Verify no route-level LLM timeout remains**

Run:

```powershell
Select-String -Path "internal/server/routes.go" -Pattern "LLMRequestTimeout"
```

Expected: no output.

- [ ] **Step 4: Run route/API tests**

Run:

```powershell
gofmt -w "internal/server/routes.go"
go test ./internal/server ./internal/server/api -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 5**

Run:

```powershell
git add "internal/server/routes.go"
git commit -m @'
feat(server): avoid route-level deadlines for llm streams

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
'@
```

---

### Task 6: Full verification, local server rebuild, browser smoke, and final commit if needed

**Files:**
- No planned file changes unless verification reveals issues.

- [ ] **Step 1: Run root module build/lint/test**

Run:

```powershell
go build ./...
golangci-lint run --timeout 10m --max-same-issues 50 ./...
go test ./...
```

Expected: all exit 0. `golangci-lint` may print the existing gomodguard deprecation warning; acceptable only if it ends with `0 issues` and exit code 0.

- [ ] **Step 2: Run llm module build/lint/test**

Run:

```powershell
Push-Location "llm"; go build ./...; $code = $LASTEXITCODE; Pop-Location; exit $code
Push-Location "llm"; golangci-lint run --timeout 10m --max-same-issues 50 ./...; $code = $LASTEXITCODE; Pop-Location; exit $code
Push-Location "llm"; go test ./...; $code = $LASTEXITCODE; Pop-Location; exit $code
```

Expected: all exit 0. Same gomodguard warning caveat applies.

- [ ] **Step 3: Rebuild the backend binary**

Run:

```powershell
go build -o axonhub.exe ./cmd/axonhub/
```

Expected: exit 0.

- [ ] **Step 4: Restart local server without committing `.exe`**

First inspect process state:

```powershell
Get-Process -Name "axonhub" -ErrorAction SilentlyContinue | Select-Object Id,ProcessName,Path
```

If an `axonhub.exe` process is running, stop only that process:

```powershell
Get-Process -Name "axonhub" -ErrorAction SilentlyContinue | Stop-Process -Force -Confirm:$false
```

Start the rebuilt binary:

```powershell
.\axonhub.exe
```

Run it in the background in Claude Code so the session can continue.

- [ ] **Step 5: Verify health endpoint from PowerShell**

Run:

```powershell
try { $resp = Invoke-WebRequest -Uri "http://localhost:8090/health" -UseBasicParsing -TimeoutSec 10; "STATUS=$($resp.StatusCode)"; $resp.Content } catch { "ERROR=$($_.Exception.Message)"; exit 1 }
```

Expected: `STATUS=200` and JSON with `"status":"healthy"`.

- [ ] **Step 6: Verify health endpoint in browser**

Use Chrome DevTools MCP:

1. Navigate to `http://localhost:8090/health`.
2. Take a text snapshot.
3. Confirm the snapshot contains `"status":"healthy"`.

Expected: browser snapshot shows healthy JSON.

- [ ] **Step 7: Check no binary is staged**

Run:

```powershell
git status --short
```

Expected: code/doc changes only. `axonhub.exe` must not appear. If it appears, update `.gitignore` before committing and do not stage the binary.

- [ ] **Step 8: Commit any final verification fixes**

If Task 6 required fixes, commit them:

```powershell
git add <changed-source-files-only>
git commit -m @'
fix: complete streaming timeout verification

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
'@
```

If there are no changes, do not create an empty commit.

---

## Self-review checklist

**Spec coverage:**
- Non-streaming requests retain total deadline: Task 4.
- Streaming requests use idle timeout: Tasks 2 and 3.
- Main `http.Server.WriteTimeout` no longer cuts SSE: Task 1.
- Mixed LLM route-group timeout removed: Task 5.
- SSE, binary, AI SDK, Gemini stream writers covered: Task 3.
- Errors distinguish idle timeout, context cancellation, and upstream failure: Tasks 2 and 3.
- Verification, rebuild, restart, browser check: Task 6.

**Placeholder scan:** No `TBD`, `TODO`, or unresolved placeholder steps. Task 3 includes implementation guidance for binary/AI SDK/Gemini writers; implementers must replace comments with complete compiling logic as explicitly stated.

**Type consistency:** `StreamWriteOptions`, `ErrStreamIdleTimeout`, `nextStreamEvent`, `WriteSSEStreamWithOptions`, `WriteJSONStreamWithOptions`, and `WriteGeminiStreamWithOptions` are introduced before use in later tasks. The plan keeps `llm/streams.Stream` unchanged and localizes concurrency to `internal/server/api/stream_timeout.go`.
