package orchestrator

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"

	"github.com/gorilla/websocket"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/requestexecution"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/llm"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

const (
	// ErrTypeUpstreamError is the error type reported when the upstream connection breaks.
	ErrTypeUpstreamError = "upstream_error"
	// ErrCodeUpstreamStreamInterrupted is the stable code for an upstream connection that
	// ended before the response completed: EOF, reset, timeout or a missing terminal event.
	ErrCodeUpstreamStreamInterrupted = "upstream_stream_interrupted"
)

// IsUpstreamTransportError reports whether err is a transport-level failure of the
// upstream connection rather than an HTTP error body or a local cancellation: the
// connection ended early (io.EOF / io.ErrUnexpectedEOF), the stream never delivered a
// terminal event, or the network reported a reset or timeout.
func IsUpstreamTransportError(err error) bool {
	if err == nil {
		return false
	}

	// Already carries a status code or structured detail: nothing to classify.
	if _, ok := errors.AsType[*httpclient.Error](err); ok {
		return false
	}

	if _, ok := errors.AsType[*llm.ResponseError](err); ok {
		return false
	}

	if errors.Is(err, httpclient.ErrStreamIdleTimeout) {
		return true
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	if errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, llm.ErrStreamIncomplete) ||
		errors.Is(err, ErrStreamIncomplete) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}

	// A WebSocket close frame from the upstream connection signals the same
	// class of failure as a reset stream for the retryable codes below.
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) && isRetryableWebSocketCloseCode(closeErr.Code) {
		return true
	}

	if isHTTP2ConnectionError(err) {
		return true
	}

	var netErr net.Error

	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}

// isRetryableWebSocketCloseCode reports whether a WebSocket close code signals
// a transient upstream failure that warrants a retry. These codes indicate the
// connection ended without a usable response rather than a client-initiated
// clean shutdown, so retrying on a fresh connection cannot duplicate output.
func isRetryableWebSocketCloseCode(code int) bool {
	switch code {
	case websocket.CloseAbnormalClosure, // 1006: no close frame, e.g. TCP reset
		websocket.CloseInternalServerErr, // 1011: server failed mid-request
		websocket.CloseServiceRestart,    // 1012: server restarting
		websocket.CloseTryAgainLater:     // 1013: temporary overload
		return true
	default:
		return false
	}
}

// isHTTP2ConnectionError reports whether err is an HTTP/2 stream reset or GOAWAY from
// the peer. net/http keeps these error types unexported (they live in its bundled h2
// copy), so the only way to recognize them is by message.
func isHTTP2ConnectionError(err error) bool {
	msg := err.Error()

	if strings.Contains(msg, "stream error: stream ID ") &&
		(strings.Contains(msg, "INTERNAL_ERROR") || strings.Contains(msg, "REFUSED_STREAM")) {
		return true
	}

	return strings.Contains(msg, "http2: server sent GOAWAY")
}

// ClassifyUpstreamTransportError converts a transport-level upstream failure into a
// *llm.ResponseError with 502 semantics, a stable code and the original cause, so
// clients and the request log see a classifiable error instead of a bare
// "unexpected EOF". Any other error is returned unchanged.
func ClassifyUpstreamTransportError(err error) error {
	if !IsUpstreamTransportError(err) {
		return err
	}

	return &llm.ResponseError{
		StatusCode: http.StatusBadGateway,
		Detail: llm.ErrorDetail{
			Message:   "Upstream provider closed the connection before the response completed: " + err.Error(),
			Type:      ErrTypeUpstreamError,
			Code:      ErrCodeUpstreamStreamInterrupted,
			Param:     "",
			RequestID: "",
		},
		Cause: err,
	}
}

// persistRequestExecutionFailure marks an execution failed (or canceled) with a classified
// error message and status code, keeping any latency metrics captured before the failure.
func persistRequestExecutionFailure(
	ctx context.Context,
	requestService *biz.RequestService,
	execution *ent.RequestExecution,
	rawErr error,
	metrics *biz.LatencyMetrics,
	upstreamModelID string,
	responseHeaders http.Header,
	chunks ...*httpclient.StreamEvent,
) error {
	status := requestexecution.StatusFailed
	if !errors.Is(rawErr, httpclient.ErrStreamIdleTimeout) &&
		(errors.Is(rawErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)) {
		status = requestexecution.StatusCanceled
	}

	failure := ClassifyUpstreamTransportError(rawErr)

	return requestService.UpdateRequestExecutionStatusWithMetrics(
		ctx,
		execution,
		status,
		ExtractErrorMessage(failure),
		ExtractErrorInfo(failure),
		metrics,
		upstreamModelID,
		responseHeaders, chunks...,
	)
}
