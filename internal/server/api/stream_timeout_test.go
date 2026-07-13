package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm/httpclient"
)

type blockingTestStream struct {
	nextCh  chan *httpclient.StreamEvent
	current *httpclient.StreamEvent
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

func TestStreamEventWaiterReturnsHeartbeatBeforeDelayedEvent(t *testing.T) {
	stream := newBlockingTestStream()
	waiter := newStreamEventWaiter(t.Context(), stream, time.Second, 10*time.Millisecond)

	result := waiter.Next()
	require.True(t, result.heartbeat)
	require.False(t, result.ok)

	want := &httpclient.StreamEvent{Type: "message", Data: []byte(`{"ok":true}`)}
	go func() {
		stream.nextCh <- want
	}()

	result = waiter.Next()
	require.True(t, result.ok)
	require.False(t, result.heartbeat)
	require.Equal(t, want, result.event)
}

func TestStreamEventWaiterHeartbeatDoesNotResetIdleTimeout(t *testing.T) {
	stream := newBlockingTestStream()
	waiter := newStreamEventWaiter(t.Context(), stream, 35*time.Millisecond, 5*time.Millisecond)
	started := time.Now()
	heartbeats := 0

	for {
		result := waiter.Next()
		if result.heartbeat {
			heartbeats++
			continue
		}

		require.ErrorIs(t, result.err, ErrStreamIdleTimeout)
		break
	}

	require.GreaterOrEqual(t, heartbeats, 2)
	require.Less(t, time.Since(started), 150*time.Millisecond)
}

func TestWriteSSEStreamWithOptionsEmitsCommentHeartbeat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	stream := newBlockingTestStream()

	go func() {
		time.Sleep(20 * time.Millisecond)
		stream.nextCh <- &httpclient.StreamEvent{Type: "message", Data: []byte(`{"ok":true}`)}
		close(stream.nextCh)
	}()

	WriteSSEStreamWithOptions(c, stream, StreamWriteOptions{
		IdleTimeout:       time.Second,
		KeepaliveInterval: 5 * time.Millisecond,
	})

	body := w.Body.String()
	require.Contains(t, body, ": keepalive\n\n")
	require.Contains(t, body, `event:message`)
	require.Contains(t, body, `data: {"ok":true}`)
}

func TestWriteSSEStreamWithOptionsDoesNotEmitHeartbeatWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	stream := newBlockingTestStream()

	go func() {
		stream.nextCh <- &httpclient.StreamEvent{Data: []byte(`{"ok":true}`)}
		close(stream.nextCh)
	}()

	WriteSSEStreamWithOptions(c, stream, StreamWriteOptions{IdleTimeout: time.Second})

	require.NotContains(t, w.Body.String(), ": keepalive")
}

func TestWriteGeminiStreamWithOptionsKeepsJSONValidWithHeartbeat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	stream := newBlockingTestStream()

	go func() {
		time.Sleep(20 * time.Millisecond)
		stream.nextCh <- &httpclient.StreamEvent{Data: []byte(`{"step":1}`)}
		close(stream.nextCh)
	}()

	WriteGeminiStreamWithOptions(c, stream, StreamWriteOptions{
		IdleTimeout:       time.Second,
		KeepaliveInterval: 5 * time.Millisecond,
	})

	var body []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, float64(1), body[0]["step"])
	require.Contains(t, w.Body.String(), "\n")
}

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
