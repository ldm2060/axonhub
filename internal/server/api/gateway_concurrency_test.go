package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/server/middleware"
	"github.com/ldm2060/axonhub/internal/server/orchestrator"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

// Use each public protocol handler, including the two Gemini route forms.
func TestNonResponsesWebSocketsShareUserConcurrency(t *testing.T) {
	for _, path := range []string{
		"/v1/chat/completions", "/v1/messages", "/anthropic/v1/messages",
		"/gemini/v1beta/models/test:streamGenerateContent", "/v1beta/models/test:streamGenerateContent",
	} {
		t.Run(path, func(t *testing.T) {
			cfg := middleware.NewConcurrencyLimitConfig(1)
			finish := make(chan struct{})
			t.Cleanup(sync.OnceFunc(func() { close(finish) }))
			h := &ChatCompletionHandlers{Processor: concurrencyTestProcessor(func(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
				return orchestrator.ChatCompletionResult{ChatCompletionStream: &concurrencyTestStream{ctx: ctx, release: finish}}, nil
			})}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Request = c.Request.WithContext(contexts.WithAPIKey(c.Request.Context(), &ent.APIKey{ID: 2, UserID: 1}))
			}, middleware.WithConcurrencyLimit(cfg))
			switch {
			case strings.Contains(path, "messages"):
				router.GET(path, (&AnthropicHandlers{ChatCompletionHandlers: h}).CreateMessageWebSocket)
			case strings.Contains(path, "models/"):
				router.GET(path, (&GeminiHandlers{ChatCompletionHandlers: h}).GenerateContentWebSocket)
			default:
				router.GET(path, (&OpenAIHandlers{ChatCompletionHandlers: h}).ChatCompletionWebSocket)
			}
			router.POST("/v1/chat/completions", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			router.GET("/v1/models", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			server := httptest.NewServer(router)
			t.Cleanup(server.Close)
			connections := make([]*websocket.Conn, 2)
			for i := range connections {
				conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+path, nil)
				require.NoError(t, err)
				connections[i] = conn
				t.Cleanup(func() { _ = conn.Close() })
			}
			requireConcurrencyIdle(t, cfg)
			require.NoError(t, connections[0].WriteMessage(websocket.TextMessage, []byte(`{"model":"test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)))
			require.Contains(t, string(readConcurrencyEvent(t, connections[0])), "response.created")
			require.NoError(t, connections[1].WriteMessage(websocket.TextMessage, []byte(`{"model":"test","stream":true}`)))
			rejected := string(readConcurrencyEvent(t, connections[1]))
			if strings.Contains(path, "models/") {
				require.Contains(t, rejected, "RESOURCE_EXHAUSTED")
			} else {
				require.Contains(t, rejected, "concurrency_limit_exceeded")
			}
			for _, method := range []string{http.MethodPost, http.MethodGet} {
				url := server.URL + "/v1/chat/completions"
				want := http.StatusTooManyRequests
				if method == http.MethodGet {
					url, want = server.URL+"/v1/models", http.StatusNoContent
				}
				req, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(`{}`))
				require.NoError(t, err)
				resp, err := server.Client().Do(req)
				require.NoError(t, err)
				_ = resp.Body.Close()
				require.Equal(t, want, resp.StatusCode)
			}
			require.NoError(t, connections[0].Close())
			requireConcurrencyIdle(t, cfg)
			require.NoError(t, connections[1].WriteMessage(websocket.TextMessage, []byte(`{"model":"test","stream":true}`)))
			require.Contains(t, string(readConcurrencyEvent(t, connections[1])), "response.created")
			require.NoError(t, connections[1].Close())
			requireConcurrencyIdle(t, cfg)
		})
	}
}

type lateProcessStream struct {
	closed chan struct{}
	once   sync.Once
}

func (s *lateProcessStream) Next() bool                       { return false }
func (s *lateProcessStream) Current() *httpclient.StreamEvent { return nil }
func (s *lateProcessStream) Err() error                       { return nil }
func (s *lateProcessStream) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func TestHTTPKeepaliveClosesAbandonedProcessStream(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", nil)
	stream := &lateProcessStream{closed: make(chan struct{})}
	finish := make(chan struct{})
	release := sync.OnceFunc(func() { close(finish) })
	t.Cleanup(release)
	processor := concurrencyTestProcessor(func(context.Context, *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		cancel()
		<-finish
		return orchestrator.ChatCompletionResult{ChatCompletionStream: stream}, nil
	})
	_, err := processWithHTTPKeepalive(c, ctx, processor, &httpclient.Request{}, time.Hour, httpStreamKeepaliveSSE, nil, "")
	require.ErrorIs(t, err, context.Canceled)
	release()
	select {
	case <-stream.closed:
	case <-time.After(time.Second):
		t.Fatal("late stream was not closed after the HTTP request returned")
	}
}

type cancellationHandoffStream struct {
	started chan struct{}
	finish  chan struct{}
	calls   atomic.Int32
	active  atomic.Bool
	raced   atomic.Bool
}

func (s *cancellationHandoffStream) Next() bool {
	if s.active.Swap(true) {
		s.raced.Store(true)
	}
	defer s.active.Store(false)
	if s.calls.Add(1) == 1 {
		close(s.started)
		<-s.finish
		return true
	}
	return false
}

func (s *cancellationHandoffStream) Current() *httpclient.StreamEvent {
	return &httpclient.StreamEvent{Data: []byte(`[DONE]`)}
}
func (s *cancellationHandoffStream) Err() error { return nil }
func (s *cancellationHandoffStream) Close() error {
	if s.active.Load() {
		s.raced.Store(true)
	}
	return nil
}

func TestCanceledStreamWaitsForReadBeforeDrainOrClose(t *testing.T) {
	for _, binary := range []bool{false, true} {
		t.Run(map[bool]string{false: "SSE", true: "binary"}[binary], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			stream := &cancellationHandoffStream{started: make(chan struct{}), finish: make(chan struct{})}
			release := sync.OnceFunc(func() { close(stream.finish) })
			t.Cleanup(release)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, "/", nil)
			done := make(chan struct{})
			go func() {
				defer close(done)
				defer func() {
					if cause := recover(); cause != nil {
						t.Errorf("stream writer panic: %v", cause)
					}
				}()
				if binary {
					WriteBinaryStreamWithOptions(c, stream, StreamWriteOptions{IdleTimeout: time.Second})
				} else {
					WriteSSEStreamWithOptions(c, stream, StreamWriteOptions{})
				}
				_ = stream.Close()
			}()
			select {
			case <-stream.started:
			case <-time.After(time.Second):
				t.Fatal("stream read never started")
			}
			cancel()
			select {
			case <-done:
				t.Error("writer returned while a stream read was still running")
			case <-time.After(20 * time.Millisecond):
			}
			release()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("stream writer did not finish after read handoff")
			}
			require.False(t, stream.raced.Load(), "Next/Close overlapped with the background read")
			if !binary {
				require.Contains(t, w.Body.String(), "[DONE]")
			}
		})
	}
}

func TestWebSocketStreamDeadlineSendsTerminalError(t *testing.T) {
	cfg := middleware.NewConcurrencyLimitConfig(1)
	server, _, _ := newConcurrencyWebSocketServer(t, cfg, func(context.Context, *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		return orchestrator.ChatCompletionResult{ChatCompletionStream: &errorAfterStream{err: context.DeadlineExceeded}}, nil
	}, false)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"test","input":"hi"}`)))
	message := readConcurrencyEvent(t, conn)
	require.Equal(t, "response.failed", gjson.GetBytes(message, "type").String())
	require.Contains(t, gjson.GetBytes(message, "response.error.message").String(), context.DeadlineExceeded.Error())
	requireConcurrencyIdle(t, cfg)
}

func TestNextStreamEventRecoversReaderPanic(t *testing.T) {
	result := nextStreamEvent(t.Context(), &panicTestStream{}, time.Second)
	require.ErrorContains(t, result.err, "stream read panic")
}

type panicTestStream struct{}

func (*panicTestStream) Next() bool                       { panic(errors.New("reader failed")) }
func (*panicTestStream) Current() *httpclient.StreamEvent { return nil }
func (*panicTestStream) Err() error                       { return nil }
func (*panicTestStream) Close() error                     { return nil }

type contextLifecycleStream struct {
	ctx        context.Context
	reading    atomic.Bool
	closed     atomic.Bool
	overlapped atomic.Bool
}

func (s *contextLifecycleStream) Next() bool {
	s.reading.Store(true)
	defer s.reading.Store(false)
	<-s.ctx.Done()
	return false
}
func (s *contextLifecycleStream) Current() *httpclient.StreamEvent { return nil }
func (s *contextLifecycleStream) Err() error                       { return s.ctx.Err() }
func (s *contextLifecycleStream) Close() error {
	if s.reading.Load() {
		s.overlapped.Store(true)
	}
	s.closed.Store(true)
	return nil
}

func TestStreamIdleTimeoutCancelsAndJoinsBeforeClose(t *testing.T) {
	for _, test := range []struct {
		name   string
		writer StreamWriter
		format sseHeartbeatFormat
	}{
		{name: "SSE", writer: WriteSSEStreamWithOptions},
		{name: "OpenAI heartbeat", format: sseHeartbeatOpenAI},
		{name: "Anthropic heartbeat", format: sseHeartbeatAnthropic},
		{name: "Gemini JSON", writer: WriteGeminiStreamWithOptions},
		{name: "AI SDK", writer: WriteJSONStreamWithOptions},
		{name: "binary", writer: WriteBinaryStreamWithOptions},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stream *contextLifecycleStream
			handler := &ChatCompletionHandlers{
				Processor: concurrencyTestProcessor(func(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
					stream = &contextLifecycleStream{ctx: ctx}
					return orchestrator.ChatCompletionResult{ChatCompletionStream: stream}, nil
				}),
				StreamWriter:       test.writer,
				StreamIdleTimeout:  25 * time.Millisecond,
				sseHeartbeatFormat: test.format,
				sseKeepAlive:       SSEKeepAliveConfig{Enabled: true, Interval: 5 * time.Millisecond},
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			// Bound the regression: the old heartbeat branch ignores idle timeout.
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			c.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", nil)
			started := time.Now()
			handler.ChatCompletionWithRequest(c, &httpclient.Request{Body: []byte(`{"model":"test","stream":true}`)})
			require.Less(t, time.Since(started), 500*time.Millisecond)
			require.True(t, stream.closed.Load())
			require.False(t, stream.overlapped.Load(), "Close must not overlap a pending read")
			if test.format != sseHeartbeatNone {
				require.Contains(t, w.Body.String(), "stream idle timeout")
				if test.format == sseHeartbeatAnthropic {
					require.Contains(t, w.Body.String(), `"type":"ping"`)
				}
			}
		})
	}
}

func TestNonSSEWriteFailureStopsConsuming(t *testing.T) {
	for name, writer := range map[string]StreamWriter{
		"Gemini JSON": WriteGeminiStreamWithOptions,
		"AI SDK":      WriteJSONStreamWithOptions,
		"binary":      WriteBinaryStreamWithOptions,
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			c.Writer = &failingResponseWriter{ResponseWriter: c.Writer, err: errors.New("broken pipe")}
			stream := &trackingStream{items: []*httpclient.StreamEvent{
				{Data: []byte(`{"text":"first"}`)},
				{Data: []byte(`{"text":"second"}`)},
				{Data: []byte(`{"text":"third"}`)},
			}}
			writer(c, stream, StreamWriteOptions{})
			require.LessOrEqual(t, stream.idx, 1, "write failure must stop reading upstream")
		})
	}
}
