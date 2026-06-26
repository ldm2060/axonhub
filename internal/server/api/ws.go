package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/orchestrator"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/streams"
)

// WSFrameMode selects how stream events are framed on an inbound WebSocket.
type WSFrameMode int

const (
	// WSFrameSSEBytes sends each stream event as one WS text message containing
	// the exact SSE event bytes (event:/data:). Used by chat / Anthropic / Gemini.
	WSFrameSSEBytes WSFrameMode = iota
	// WSFrameResponsesEvents sends each event's JSON object (with its top-level
	// "type" field) as one WS text message, mirroring the outbound Responses WS.
	WSFrameResponsesEvents
)

// inboundWSUpgrader upgrades inbound model-endpoint WebSocket connections.
var inboundWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// prepareWSRequest converts an inbound WS message into the request body bytes
// expected by the pipeline for the given frame mode.
func prepareWSRequest(mode WSFrameMode, msg []byte) ([]byte, error) {
	if len(msg) == 0 {
		return nil, errors.New("request message is empty")
	}

	payload := map[string]any{}
	if err := json.Unmarshal(msg, &payload); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}

	if mode == WSFrameResponsesEvents {
		// Strip the response.create envelope and force streaming on (Responses
		// WS is inherently event-streamed). Non-Responses bodies pass through
		// unchanged so provider-specific top-level fields are preserved.
		delete(payload, "type")
		payload["stream"] = true
	}

	return json.Marshal(payload)
}

// buildWSGenericRequest builds a pipeline-ready generic request from the WS
// upgrade request and the prepared body bytes. Method/path/query/headers come
// from the upgrade request; the body comes from the WS message.
func buildWSGenericRequest(upgrade *http.Request, body []byte) (*httpclient.Request, error) {
	req := upgrade.Clone(upgrade.Context())
	req.Method = http.MethodPost
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return httpclient.ReadHTTPRequest(req)
}

// writeWSResponse writes a non-streaming response as a single WS message.
func writeWSResponse(conn *websocket.Conn, resp *httpclient.Response) error {
	if resp == nil {
		return nil
	}

	return conn.WriteMessage(websocket.TextMessage, resp.Body)
}

// writeWSStream drains the stream and writes each event to the WS connection
// according to mode. It returns when the stream ends, errors out, or the
// connection is no longer writable. The stream is always closed.
func writeWSStream(ctx context.Context, conn *websocket.Conn, stream streams.Stream[*httpclient.StreamEvent], mode WSFrameMode, idleTimeout time.Duration) {
	defer func() {
		if err := stream.Close(); err != nil {
			log.Debug(ctx, "close ws stream", log.Cause(err))
		}
	}()

	for {
		result := nextStreamEvent(ctx, stream, idleTimeout)
		if result.ok {
			if err := writeWSStreamEvent(conn, result.event, mode); err != nil {
				log.Warn(ctx, "ws write failed, stopping stream", log.Cause(err))
				return
			}

			continue
		}

		if result.err != nil {
			if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
				log.Warn(ctx, "ws stream context done", log.Cause(result.err))
				return
			}

			log.Warn(ctx, "ws stream error", log.Cause(result.err))
			_ = writeWSStreamEvent(conn, &httpclient.StreamEvent{
				Type: "error",
				Data: wsErrorEventData(ctx, mode, result.err),
			}, mode)
		}

		return
	}
}

// writeWSStreamEvent encodes one stream event as a WS text message.
func writeWSStreamEvent(conn *websocket.Conn, ev *httpclient.StreamEvent, mode WSFrameMode) error {
	if ev == nil {
		return nil
	}

	switch mode {
	case WSFrameResponsesEvents:
		if len(ev.Data) == 0 {
			return nil
		}

		return conn.WriteMessage(websocket.TextMessage, ev.Data)
	default:
		var buf bytes.Buffer
		if err := sse.Encode(&buf, sse.Event{Event: ev.Type, Data: ev.Data}); err != nil {
			return err
		}

		return conn.WriteMessage(websocket.TextMessage, buf.Bytes())
	}
}

// wsErrorEventData returns the error-event payload bytes appropriate for the mode.
func wsErrorEventData(ctx context.Context, mode WSFrameMode, err error) []byte {
	if mode == WSFrameResponsesEvents {
		body, _ := json.Marshal(map[string]any{
			"type":    "error",
			"code":    "server_error",
			"message": orchestrator.ExtractErrorMessage(err),
		})
		return body
	}

	body, _ := json.Marshal(FormatStreamError(ctx, err))
	return body
}

// startWSKeepalive sends a PING control frame every interval until the returned
// stop func is called or the context is cancelled. WriteControl is safe for
// concurrent use with WriteMessage. Returns a no-op stop func when interval <= 0.
func startWSKeepalive(ctx context.Context, conn *websocket.Conn, interval time.Duration) (stop func()) {
	if interval <= 0 {
		return func() {}
	}

	done := make(chan struct{})

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Warn(ctx, "ws keepalive panic recovered", log.Any("panic", rec))
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			}
		}
	}()

	return func() { close(done) }
}

// keepaliveInterval reads the configured WS keepalive interval from system settings.
func (h *ChatCompletionHandlers) keepaliveInterval(ctx context.Context) time.Duration {
	svc := h.systemService()
	if svc == nil {
		return 0
	}

	settings, err := svc.StreamingSettings(ctx)
	if err != nil || settings == nil {
		return 0
	}

	return time.Duration(settings.WebSocketKeepaliveIntervalSeconds) * time.Second
}

// ChatCompletionWebSocket handles an inbound WebSocket connection that mirrors
// the HTTP streaming endpoint. Each inbound text message is one request; the
// response is streamed back as WS messages. The connection is kept open for
// multiple serial requests until the client disconnects.
func (h *ChatCompletionHandlers) ChatCompletionWebSocket(c *gin.Context, mode WSFrameMode) {
	conn, err := inboundWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade already wrote the HTTP error response.
		return
	}

	defer func() {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
	}()

	ctx := c.Request.Context()
	stop := startWSKeepalive(ctx, conn, h.keepaliveInterval(ctx))
	defer stop()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return // client closed or read error
		}

		if exit := h.handleWSRequest(ctx, conn, c, msg, mode); exit {
			return
		}
	}
}

// handleWSRequest processes one WS request message. It returns true if the
// connection should be closed (write failure), false to continue the loop.
func (h *ChatCompletionHandlers) handleWSRequest(ctx context.Context, conn *websocket.Conn, c *gin.Context, msg []byte, mode WSFrameMode) bool {
	body, err := prepareWSRequest(mode, msg)
	if err != nil {
		log.Warn(ctx, "ws request prepare error", log.Cause(err))
		return h.writeWSRequestError(conn, ctx, mode, err)
	}

	genericReq, err := buildWSGenericRequest(c.Request, body)
	if err != nil {
		log.Warn(ctx, "ws request build error", log.Cause(err))
		return h.writeWSRequestError(conn, ctx, mode, err)
	}

	result, err := h.processor().Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "ws process error", log.Cause(err))
		return h.writeWSRequestError(conn, ctx, mode, err)
	}

	if result.ChatCompletion != nil {
		if werr := writeWSResponse(conn, result.ChatCompletion); werr != nil {
			return true
		}

		return false
	}

	if result.ChatCompletionStream != nil {
		stream := newUpstreamErrorStream(ctx, result.ChatCompletionStream, h.systemService())
		writeWSStream(ctx, conn, stream, mode, h.StreamIdleTimeout)
		return false
	}

	// No response object and no stream: nothing to send for this request.
	return false
}

// writeWSRequestError sends a single error frame for a failed request.
// Returns true if the write failed (caller should close the connection).
func (h *ChatCompletionHandlers) writeWSRequestError(conn *websocket.Conn, ctx context.Context, mode WSFrameMode, err error) bool {
	var data []byte
	if h.ChatCompletionOrchestrator != nil {
		httpErr := transformOrchestratorError(ctx, err, h.ChatCompletionOrchestrator)
		if mode == WSFrameResponsesEvents {
			obj := map[string]any{
				"type":    "error",
				"code":    "server_error",
				"message": orchestrator.ExtractErrorMessage(err),
			}
			data, _ = json.Marshal(obj)
		} else {
			data = httpErr.Body
		}
	} else {
		data = wsErrorEventData(ctx, mode, err)
	}

	if werr := writeWSStreamEvent(conn, &httpclient.StreamEvent{Type: "error", Data: data}, mode); werr != nil {
		return true
	}

	return false
}
