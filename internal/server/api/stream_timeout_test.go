package api

import (
	"context"
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
