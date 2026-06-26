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

		stop := startWSKeepalive(r.Context(), conn, 50*time.Millisecond)
		defer stop()
		_, _, _ = conn.ReadMessage() // block until closed
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server, ""), nil)
	require.NoError(t, err)
	defer conn.Close()

	got := make(chan struct{}, 4)
	conn.SetPingHandler(func(string) error { got <- struct{}{}; return nil })

	// Client must be actively reading for control frame handlers to fire.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

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
