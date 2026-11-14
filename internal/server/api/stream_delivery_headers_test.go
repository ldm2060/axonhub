package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/streams"
)

type deliveryRawFlushWriter struct {
	*httptest.ResponseRecorder

	flushes int
}

func (w *deliveryRawFlushWriter) FlushError() error {
	w.flushes++
	if w.flushes > 1 {
		return io.ErrClosedPipe
	}
	w.ResponseRecorder.Flush()
	return nil
}

// deliveryHeaderCaptureWriter stands in for the response-header capture wrapper
// that used to sit at the HTTP boundary: it must forward FlushError so SSE
// disconnect detection keeps working through an extra writer layer.
type deliveryHeaderCaptureWriter struct {
	gin.ResponseWriter
}

func (w *deliveryHeaderCaptureWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *deliveryHeaderCaptureWriter) FlushError() error {
	w.ResponseWriter.WriteHeaderNow()
	var writer http.ResponseWriter = w.ResponseWriter
	for {
		if flusher, ok := writer.(interface{ FlushError() error }); ok {
			return flusher.FlushError()
		}
		if unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter }); ok {
			writer = unwrapper.Unwrap()
			continue
		}
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
			return nil
		}
		return http.ErrNotSupported
	}
}

func TestStreamDeliverySSEHeadersWrapperFlushError(t *testing.T) {
	// Given: middleware wraps Gin, which wraps an error-returning transport.
	w := &deliveryRawFlushWriter{ResponseRecorder: httptest.NewRecorder()}
	s := &deliverySSEStream{trace: &deliverySSETrace{}, Stream: streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
	})}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithRequestRecordObserver(c.Request.Context(), func(int) {}))
		c.Writer = &deliveryHeaderCaptureWriter{ResponseWriter: c.Writer}
		c.Next()
	})
	engine.GET("/", func(c *gin.Context) { WriteSSEStream(c, s) })
	// When
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	// Then: the event flush error survives both wrappers and cannot acknowledge.
	require.Equal(t, 2, w.flushes)
	require.False(t, s.DeliveryAcknowledged())
}
