package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	"github.com/ldm2060/axonhub/llm/transformer/openai/responses"
)

type concurrencyTestProcessor responsesWebSocketProcessFunc

func (p concurrencyTestProcessor) Process(ctx context.Context, req *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
	return p(ctx, req)
}

// The processor returns immediately, but generation stays active until the
// stream is drained. This catches releasing a slot as soon as Process returns.
type concurrencyTestStream struct {
	ctx     context.Context
	release <-chan struct{}
	index   int
}

func (s *concurrencyTestStream) Next() bool {
	s.index++
	if s.index == 2 {
		select {
		case <-s.release:
		case <-s.ctx.Done():
			return false
		}
	}
	return s.index <= 2
}

func (s *concurrencyTestStream) Current() *httpclient.StreamEvent {
	if s.index == 1 {
		return &httpclient.StreamEvent{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_test"}}`)}
	}
	return &httpclient.StreamEvent{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_test","status":"completed"}}`)}
}

func (s *concurrencyTestStream) Err() error   { return s.ctx.Err() }
func (s *concurrencyTestStream) Close() error { return nil }

func newConcurrencyWebSocketServer(t *testing.T, cfg *middleware.ConcurrencyLimitConfig, process responsesWebSocketProcessFunc, dispatcher bool) (*httptest.Server, <-chan struct{}, func()) {
	t.Helper()
	postEntered := make(chan struct{}, 1)
	postRelease := make(chan struct{})
	releasePost := sync.OnceFunc(func() { close(postRelease) })
	router := gin.New()
	router.Use(func(c *gin.Context) {
		userID := 1
		if c.GetHeader("X-Test-User") == "other" {
			userID = 2
		}
		c.Request = c.Request.WithContext(contexts.WithPrincipalUser(c.Request.Context(), &ent.User{ID: userID}))
	}, middleware.WithConcurrencyLimit(cfg))
	if dispatcher {
		router.GET("/v1/responses", func(c *gin.Context) { serveResponsesWebSocket(c, time.Second*5, process, nil) })
	} else {
		// Exercise the handler actually registered by the production router.
		handlers := &OpenAIHandlers{ResponseCompletionHandlers: &ChatCompletionHandlers{
			Processor:                  concurrencyTestProcessor(process),
			ChatCompletionOrchestrator: &orchestrator.ChatCompletionOrchestrator{Inbound: responses.NewInboundTransformer()},
		}}
		router.GET("/v1/responses", handlers.CreateResponseWebSocket)
	}
	router.POST("/v1/responses", func(c *gin.Context) {
		if c.GetHeader("X-Test-Hold") == "true" {
			postEntered <- struct{}{}
			select {
			case <-postRelease:
			case <-c.Request.Context().Done():
			}
		}
		c.Status(http.StatusNoContent)
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	t.Cleanup(releasePost)
	return server, postEntered, releasePost
}

func readConcurrencyEvent(t *testing.T, conn *websocket.Conn) []byte {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, message, err := conn.ReadMessage()
	require.NoError(t, err)
	return message
}

func requireConcurrencyIdle(t *testing.T, cfg *middleware.ConcurrencyLimitConfig) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, tracked, _ := cfg.Stats()
		return tracked == 0
	}, 5*time.Second, time.Millisecond)
}

func TestWebSocketConcurrencySharedWithHTTP(t *testing.T) {
	for _, dispatcher := range []bool{false, true} {
		name := "production_handler"
		if dispatcher {
			name = "responses_dispatcher"
		}
		t.Run(name, func(t *testing.T) {
			cfg := middleware.NewConcurrencyLimitConfig(0)
			finish := make(chan struct{})
			finishStream := sync.OnceFunc(func() { close(finish) })
			t.Cleanup(finishStream)
			process := func(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
				return orchestrator.ChatCompletionResult{ChatCompletionStream: &concurrencyTestStream{ctx: ctx, release: finish}}, nil
			}
			server, postEntered, releasePost := newConcurrencyWebSocketServer(t, cfg, process, dispatcher)
			connections := make([]*websocket.Conn, 0, 7)
			for range 7 {
				conn := dialResponsesWebSocket(t, server.URL, nil)
				t.Cleanup(func() { _ = conn.Close() })
				connections = append(connections, conn)
			}
			requireConcurrencyIdle(t, cfg)
			cfg.Apply(2)

			post := func(user string, want int) {
				t.Helper()
				req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/v1/responses", strings.NewReader(`{}`))
				require.NoError(t, err)
				req.Header.Set("X-Test-User", user)
				resp, err := server.Client().Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				require.Equal(t, want, resp.StatusCode)
				if want == http.StatusTooManyRequests {
					require.Equal(t, "1", resp.Header.Get("Retry-After"))
				}
			}
			send := func(conn *websocket.Conn) {
				t.Helper()
				require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","input":"hello"}`)))
			}
			denied := func(conn *websocket.Conn) {
				t.Helper()
				send(conn)
				event := readConcurrencyEvent(t, conn)
				if dispatcher {
					require.Equal(t, "error", gjson.GetBytes(event, "type").String())
					require.Equal(t, int64(429), gjson.GetBytes(event, "status").Int())
					require.Equal(t, "concurrency_limit_exceeded", gjson.GetBytes(event, "error.code").String())
				} else {
					require.Equal(t, "response.failed", gjson.GetBytes(event, "type").String())
					require.Equal(t, "concurrency_limit_exceeded", gjson.GetBytes(event, "response.error.code").String())
				}
			}

			send(connections[0])
			require.Equal(t, "response.created", gjson.GetBytes(readConcurrencyEvent(t, connections[0]), "type").String())
			post("", http.StatusNoContent)
			send(connections[1])
			require.Equal(t, "response.created", gjson.GetBytes(readConcurrencyEvent(t, connections[1]), "type").String())
			post("", http.StatusTooManyRequests)
			post("other", http.StatusNoContent)
			denied(connections[2])
			// Even at full generation capacity, a fresh idle connection can open.
			extra := dialResponsesWebSocket(t, server.URL, nil)
			t.Cleanup(func() { _ = extra.Close() })

			cfg.Apply(1)
			post("", http.StatusTooManyRequests)
			cfg.Apply(3)
			post("", http.StatusNoContent)
			cfg.Apply(2)
			post("", http.StatusTooManyRequests)
			finishStream()
			for _, conn := range connections[:2] {
				require.Equal(t, "response.completed", gjson.GetBytes(readConcurrencyEvent(t, conn), "type").String())
			}
			requireConcurrencyIdle(t, cfg)
			// Reuse the same socket after its earlier 429; it was not closed.
			send(connections[2])
			readConcurrencyEvent(t, connections[2])
			require.Equal(t, "response.completed", gjson.GetBytes(readConcurrencyEvent(t, connections[2]), "type").String())
			requireConcurrencyIdle(t, cfg)

			cfg.Apply(1)
			postResult := make(chan int, 1)
			var wg sync.WaitGroup
			wg.Go(func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("panic in HTTP concurrency fixture: %v", recovered)
						postResult <- 0
					}
				}()
				req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/v1/responses", nil)
				if err != nil {
					postResult <- 0
					return
				}
				req.Header.Set("X-Test-Hold", "true")
				resp, err := server.Client().Do(req)
				if err != nil {
					postResult <- 0
					return
				}
				defer resp.Body.Close()
				postResult <- resp.StatusCode
			})
			select {
			case <-postEntered:
			case <-time.After(5 * time.Second):
				t.Fatal("HTTP request did not start")
			}
			denied(connections[2])
			releasePost()
			wg.Wait()
			require.Equal(t, http.StatusNoContent, <-postResult)
			requireConcurrencyIdle(t, cfg)
		})
	}
}

func TestWebSocketConcurrencyReleasesAfterProcessError(t *testing.T) {
	for _, dispatcher := range []bool{false, true} {
		cfg := middleware.NewConcurrencyLimitConfig(1)
		process := func(context.Context, *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
			return orchestrator.ChatCompletionResult{}, errors.New("upstream failed")
		}
		server, _, _ := newConcurrencyWebSocketServer(t, cfg, process, dispatcher)
		conn := dialResponsesWebSocket(t, server.URL, nil)
		for range 2 {
			require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test"}`)))
			event := readConcurrencyEvent(t, conn)
			require.NotContains(t, string(event), "concurrency_limit_exceeded")
			requireConcurrencyIdle(t, cfg)
		}
		require.NoError(t, conn.Close())
	}
}

func TestResponsesDispatcherConcurrencyCountsLanesButNotWarmup(t *testing.T) {
	cfg := middleware.NewConcurrencyLimitConfig(1)
	finish := make(chan struct{})
	finishStream := sync.OnceFunc(func() { close(finish) })
	defer finishStream()
	requests := make(chan *httpclient.Request, 2)
	process := func(ctx context.Context, req *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		requests <- req
		return orchestrator.ChatCompletionResult{ChatCompletionStream: &concurrencyTestStream{ctx: ctx, release: finish}}, nil
	}
	server, _, _ := newConcurrencyWebSocketServer(t, cfg, process, true)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","stream_id":"active","model":"gpt-test"}`)))
	readConcurrencyEvent(t, conn)
	<-requests
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","stream_id":"warmup","model":"gpt-test","input":"prefix","generate":false}`)))
	var warmupID string
	for range 3 {
		event := readConcurrencyEvent(t, conn)
		require.Equal(t, "warmup", gjson.GetBytes(event, "stream_id").String())
		require.NotEqual(t, "error", gjson.GetBytes(event, "type").String())
		warmupID = gjson.GetBytes(event, "response.id").String()
	}
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","stream_id":"excess","model":"gpt-test"}`)))
	event := readConcurrencyEvent(t, conn)
	require.Equal(t, "excess", gjson.GetBytes(event, "stream_id").String())
	require.Equal(t, int64(429), gjson.GetBytes(event, "status").Int())

	continuation := []byte(`{"type":"response.create","stream_id":"warmup","model":"gpt-test","input":"suffix","previous_response_id":"` + warmupID + `"}`)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, continuation))
	event = readConcurrencyEvent(t, conn)
	require.Equal(t, int64(429), gjson.GetBytes(event, "status").Int())
	finishStream()
	require.Equal(t, "response.completed", gjson.GetBytes(readConcurrencyEvent(t, conn), "type").String())
	requireConcurrencyIdle(t, cfg)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, continuation))
	readConcurrencyEvent(t, conn)
	require.Equal(t, "response.completed", gjson.GetBytes(readConcurrencyEvent(t, conn), "type").String())
	req := <-requests
	require.Equal(t, "prefix", gjson.GetBytes(req.Body, "input.0.content").String())
	require.Equal(t, "suffix", gjson.GetBytes(req.Body, "input.1.content").String())
	require.False(t, gjson.GetBytes(req.Body, "previous_response_id").Exists())
	requireConcurrencyIdle(t, cfg)
}

func TestWebSocketConcurrencyDisconnectReleasesActiveStream(t *testing.T) {
	for _, dispatcher := range []bool{false, true} {
		cfg := middleware.NewConcurrencyLimitConfig(1)
		process := func(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
			return orchestrator.ChatCompletionResult{ChatCompletionStream: &concurrencyTestStream{ctx: ctx}}, nil
		}
		server, _, _ := newConcurrencyWebSocketServer(t, cfg, process, dispatcher)
		conn := dialResponsesWebSocket(t, server.URL, nil)
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test"}`)))
		require.Equal(t, "response.created", gjson.GetBytes(readConcurrencyEvent(t, conn), "type").String())
		require.NoError(t, conn.Close())
		requireConcurrencyIdle(t, cfg)
	}
}
