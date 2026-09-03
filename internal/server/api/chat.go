package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/internal/server/orchestrator"
	"github.com/ldm2060/axonhub/llm"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/streams"
)

const (
	errTypeQuotaExhausted = "quota_exhausted"
	errCodeQuotaExhausted = "quota_exhausted"
)

// StreamWriter is a function type for writing stream events to the response.
type StreamWriter func(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions)

type ChatCompletionProcessor interface {
	Process(ctx context.Context, genericReq *httpclient.Request) (orchestrator.ChatCompletionResult, error)
}

type HTTPStreamKeepaliveMode uint8

const (
	httpStreamKeepaliveDefault HTTPStreamKeepaliveMode = iota
	httpStreamKeepaliveSSE
	httpStreamKeepaliveJSONWhitespace
	httpStreamKeepaliveTextWhitespace
	httpStreamKeepaliveDisabled
)

// SSEKeepAliveConfig controls downstream heartbeats for SSE-compatible APIs.
type SSEKeepAliveConfig struct {
	Enabled  bool
	Interval time.Duration
}

type sseHeartbeatFormat uint8

const (
	sseHeartbeatNone sseHeartbeatFormat = iota
	sseHeartbeatOpenAI
	sseHeartbeatAnthropic
)

type ChatCompletionHandlers struct {
	ChatCompletionOrchestrator *orchestrator.ChatCompletionOrchestrator
	Processor                  ChatCompletionProcessor
	StreamWriter               StreamWriter
	RequestTimeout             time.Duration
	StreamIdleTimeout          time.Duration
	HTTPKeepaliveInterval      time.Duration
	HTTPKeepaliveMode          HTTPStreamKeepaliveMode
	sseKeepAlive               SSEKeepAliveConfig
	sseHeartbeatFormat         sseHeartbeatFormat
}

func NewChatCompletionHandlers(orchestrator *orchestrator.ChatCompletionOrchestrator) *ChatCompletionHandlers {
	return &ChatCompletionHandlers{
		ChatCompletionOrchestrator: orchestrator,
		Processor:                  orchestrator,
		StreamWriter:               WriteSSEStreamWithOptions,
		HTTPKeepaliveMode:          httpStreamKeepaliveSSE,
	}
}

func (handlers *ChatCompletionHandlers) processor() ChatCompletionProcessor {
	if handlers.Processor != nil {
		return handlers.Processor
	}

	return handlers.ChatCompletionOrchestrator
}

func (handlers *ChatCompletionHandlers) systemService() *biz.SystemService {
	if handlers.ChatCompletionOrchestrator == nil {
		return nil
	}

	return handlers.ChatCompletionOrchestrator.SystemService
}

func keepaliveModeForWriter(writer StreamWriter) HTTPStreamKeepaliveMode {
	if writer == nil {
		return httpStreamKeepaliveSSE
	}

	pointer := reflect.ValueOf(writer).Pointer()
	switch pointer {
	case reflect.ValueOf(WriteSSEStreamWithOptions).Pointer():
		return httpStreamKeepaliveSSE
	case reflect.ValueOf(WriteGeminiStreamWithOptions).Pointer():
		return httpStreamKeepaliveJSONWhitespace
	case reflect.ValueOf(WriteJSONStreamWithOptions).Pointer():
		return httpStreamKeepaliveTextWhitespace
	case reflect.ValueOf(WriteBinaryStreamWithOptions).Pointer():
		return httpStreamKeepaliveDisabled
	default:
		return httpStreamKeepaliveDisabled
	}
}

// WithStreamWriter returns a new ChatCompletionHandlers with the specified stream writer.
func (handlers *ChatCompletionHandlers) WithStreamWriter(writer StreamWriter) *ChatCompletionHandlers {
	return &ChatCompletionHandlers{
		ChatCompletionOrchestrator: handlers.ChatCompletionOrchestrator,
		Processor:                  handlers.processor(),
		StreamWriter:               writer,
		RequestTimeout:             handlers.RequestTimeout,
		StreamIdleTimeout:          handlers.StreamIdleTimeout,
		HTTPKeepaliveInterval:      handlers.HTTPKeepaliveInterval,
		HTTPKeepaliveMode:          keepaliveModeForWriter(writer),
		sseKeepAlive:               handlers.sseKeepAlive,
		sseHeartbeatFormat:         handlers.sseHeartbeatFormat,
	}
}

func (handlers *ChatCompletionHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	// Use ReadHTTPRequest to parse the request
	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		httpErr := handlers.ChatCompletionOrchestrator.Inbound.TransformError(ctx, err)
		c.JSON(httpErr.StatusCode, json.RawMessage(httpErr.Body))

		return
	}

	handlers.ChatCompletionWithRequest(c, genericReq)
}

func requestBodyWantsStream(genericReq *httpclient.Request) bool {
	if genericReq == nil {
		return false
	}

	if strings.Contains(strings.ToLower(genericReq.Path), ":streamgeneratecontent") {
		return true
	}

	if len(genericReq.Body) == 0 {
		return false
	}

	if gjson.GetBytes(genericReq.Body, "stream").Bool() {
		return true
	}

	return strings.TrimSpace(gjson.GetBytes(genericReq.Body, "stream_format").String()) != ""
}

type processResult struct {
	result orchestrator.ChatCompletionResult
	err    error
}

func (handlers *ChatCompletionHandlers) effectiveHTTPKeepaliveMode() HTTPStreamKeepaliveMode {
	if handlers.HTTPKeepaliveMode != httpStreamKeepaliveDefault {
		return handlers.HTTPKeepaliveMode
	}

	return keepaliveModeForWriter(handlers.StreamWriter)
}

func keepalivePayload(mode HTTPStreamKeepaliveMode) ([]byte, string) {
	switch mode {
	case httpStreamKeepaliveSSE:
		return []byte(": keepalive\n\n"), sse.ContentType
	case httpStreamKeepaliveJSONWhitespace:
		return []byte("[\n"), "application/json; charset=UTF-8"
	case httpStreamKeepaliveTextWhitespace:
		return []byte("\n"), "text/plain; charset=utf-8"
	default:
		return nil, ""
	}
}

func (handlers *ChatCompletionHandlers) configuredHTTPKeepaliveInterval(ctx context.Context) time.Duration {
	if handlers.HTTPKeepaliveInterval > 0 {
		return handlers.HTTPKeepaliveInterval
	}

	svc := handlers.systemService()
	if svc == nil {
		return 0
	}
	settings, err := svc.StreamingSettingsForRuntime(ctx)
	if err != nil || settings == nil || settings.HTTPStreamKeepaliveIntervalSeconds <= 0 {
		return 0
	}

	return time.Duration(settings.HTTPStreamKeepaliveIntervalSeconds) * time.Second
}

func processWithHTTPKeepalive(
	c *gin.Context,
	ctx context.Context,
	processor ChatCompletionProcessor,
	genericReq *httpclient.Request,
	interval time.Duration,
	mode HTTPStreamKeepaliveMode,
	payload []byte,
	contentType string,
) (orchestrator.ChatCompletionResult, error) {
	resultCh := make(chan processResult, 1)
	go func() {
		defer func() {
			if cause := recover(); cause != nil {
				log.Warn(ctx, "Chat completion process panic recovered", log.Any("panic", cause))
				resultCh <- processResult{err: fmt.Errorf("chat completion process panic: %v", cause)}
			}
		}()

		result, err := processor.Process(ctx, genericReq)
		resultCh <- processResult{result: result, err: err}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	wroteKeepalive := false

	for {
		select {
		case <-ctx.Done():
			return orchestrator.ChatCompletionResult{}, ctx.Err()
		case processed := <-resultCh:
			return processed.result, processed.err
		case <-ticker.C:
			if contentType != "" {
				c.Header("Content-Type", contentType)
			}
			c.Header("Cache-Control", "no-cache, no-transform")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")
			if mode == httpStreamKeepaliveJSONWhitespace && wroteKeepalive {
				payload = []byte("\n")
			}
			if _, err := c.Writer.Write(payload); err != nil {
				return orchestrator.ChatCompletionResult{}, err
			}
			wroteKeepalive = true
			c.Writer.Flush()
		}
	}
}

func (handlers *ChatCompletionHandlers) writeCommittedProcessError(c *gin.Context, ctx context.Context, err error, mode HTTPStreamKeepaliveMode) {
	switch mode {
	case httpStreamKeepaliveSSE:
		c.SSEvent("error", FormatStreamError(ctx, err))
		c.Writer.Flush()
	case httpStreamKeepaliveJSONWhitespace:
		body, marshalErr := json.Marshal(FormatStreamError(ctx, err))
		if marshalErr != nil {
			log.Warn(ctx, "Failed to marshal committed stream error", log.Cause(marshalErr))
			return
		}
		if _, writeErr := c.Writer.Write(body); writeErr != nil {
			log.Warn(ctx, "Failed to write committed stream error", log.Cause(writeErr))
			return
		}
		if _, writeErr := c.Writer.Write([]byte("]")); writeErr != nil {
			log.Warn(ctx, "Failed to close committed Gemini stream", log.Cause(writeErr))
			return
		}
		c.Writer.Flush()
	case httpStreamKeepaliveTextWhitespace:
		if _, writeErr := c.Writer.Write([]byte("3:" + strconv.Quote(err.Error()) + "\n")); writeErr != nil {
			log.Warn(ctx, "Failed to write committed stream error", log.Cause(writeErr))
			return
		}
		c.Writer.Flush()
	}
}

func (handlers *ChatCompletionHandlers) ChatCompletionWithRequest(c *gin.Context, genericReq *httpclient.Request) {
	ctx := c.Request.Context()

	if genericReq == nil || len(genericReq.Body) == 0 {
		JSONError(c, http.StatusBadRequest, errors.New("Request body is empty"))
		return
	}

	streaming := requestBodyWantsStream(genericReq)
	if !streaming && handlers.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, handlers.RequestTimeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
	}

	// log.Debug(ctx, "Chat completion request", log.Any("request", genericReq))

	keepaliveMode := handlers.effectiveHTTPKeepaliveMode()
	keepalivePayload, keepaliveContentType := keepalivePayload(keepaliveMode)
	keepaliveInterval := time.Duration(0)
	responseCommittedByKeepalive := false
	if streaming && len(keepalivePayload) > 0 {
		keepaliveInterval = handlers.configuredHTTPKeepaliveInterval(ctx)
	}

	var (
		result orchestrator.ChatCompletionResult
		err    error
	)
	if keepaliveInterval > 0 {
		result, err = processWithHTTPKeepalive(
			c,
			ctx,
			handlers.processor(),
			genericReq,
			keepaliveInterval,
			keepaliveMode,
			keepalivePayload,
			keepaliveContentType,
		)
		responseCommittedByKeepalive = c.Writer.Written()
	} else {
		result, err = handlers.processor().Process(ctx, genericReq)
	}
	if err != nil {
		log.Error(ctx, "Error processing chat completion", log.Cause(err))
		if c.Writer.Written() {
			handlers.writeCommittedProcessError(c, ctx, err, keepaliveMode)
			return
		}

		httpErr := transformOrchestratorError(ctx, err, handlers.ChatCompletionOrchestrator)
		c.JSON(httpErr.StatusCode, json.RawMessage(httpErr.Body))

		return
	}

	if result.ChatCompletion != nil {
		resp := result.ChatCompletion

		contentType := "application/json"
		if ct := resp.Headers.Get("Content-Type"); ct != "" {
			contentType = ct
		}

		c.Data(resp.StatusCode, contentType, resp.Body)

		return
	}

	if result.ChatCompletionStream != nil {
		defer func() {
			log.Debug(ctx, "Close chat stream")

			err := result.ChatCompletionStream.Close()
			if err != nil {
				logger.Error(ctx, "Error closing stream", log.Cause(err))
			}
		}()

		c.Header("Access-Control-Allow-Origin", "*")

		stream := newUpstreamErrorStream(ctx, result.ChatCompletionStream, handlers.systemService())

		// When per-API SSE keep-alive is enabled, use the heartbeat-aware SSE
		// writer (upstream 2d7d7c86). Otherwise use the configured stream writer
		// with our existing StreamWriteOptions flow.
		if handlers.sseKeepAlive.Enabled && handlers.sseKeepAlive.Interval > 0 && handlers.sseHeartbeatFormat != sseHeartbeatNone {
			writeSSEStream(c, stream, FormatStreamError, handlers.sseKeepAlive, handlers.sseHeartbeatFormat)
			return
		}

		streamWriter := handlers.StreamWriter
		if streamWriter == nil {
			streamWriter = WriteSSEStreamWithOptions
		}

		streamWriter(c, stream, StreamWriteOptions{
			IdleTimeout:              handlers.StreamIdleTimeout,
			KeepaliveInterval:        keepaliveInterval,
			ResponseAlreadyCommitted: responseCommittedByKeepalive,
		})
	}
}

// StreamErrorFormatter formats a stream error into a JSON-serializable object for SSE error events.
type StreamErrorFormatter func(ctx context.Context, err error) any

// maxStreamEventsAfterCancel bounds how many events the stream writers drain after
// the request context is canceled. Draining lets persistence wrappers observe a
// buffered terminal event, but streams are expected to end promptly on cancellation;
// the cap only guards against implementations that ignore it.
const maxStreamEventsAfterCancel = 256

// WriteSSEStream writes stream events as Server-Sent Events (SSE) with default error formatting.
func WriteSSEStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	WriteSSEStreamWithOptionsAndErrorFormatter(c, stream, StreamWriteOptions{}, FormatStreamError)
}

func WriteSSEStreamWithOptions(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions) {
	WriteSSEStreamWithOptionsAndErrorFormatter(c, stream, opts, FormatStreamError)
}

// WriteSSEStreamWithErrorFormatter writes stream events as SSE with a custom error formatter.
func WriteSSEStreamWithErrorFormatter(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], formatErr StreamErrorFormatter) {
	WriteSSEStreamWithOptionsAndErrorFormatter(c, stream, StreamWriteOptions{}, formatErr)
}

func WriteSSEStreamWithOptionsAndErrorFormatter(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions, formatErr StreamErrorFormatter) {
	writeSSEStreamWithoutHeartbeat(c, stream, opts, formatErr)
}

// writeSSEStream dispatches between the heartbeat-aware SSE writer and the
// default writer based on the keep-alive configuration.
func writeSSEStream(
	c *gin.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	formatErr StreamErrorFormatter,
	keepAlive SSEKeepAliveConfig,
	heartbeatFormat sseHeartbeatFormat,
) {
	if !keepAlive.Enabled || keepAlive.Interval <= 0 || heartbeatFormat == sseHeartbeatNone {
		writeSSEStreamWithoutHeartbeat(c, stream, StreamWriteOptions{}, formatErr)
		return
	}

	writeSSEStreamWithHeartbeat(c, stream, formatErr, keepAlive.Interval, heartbeatFormat)
}

func writeSSEStreamWithoutHeartbeat(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions, formatErr StreamErrorFormatter) {
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

	// Set SSE headers
	setSSEHeaders(c)
	c.Writer.Flush()

	waiter := newStreamEventWaiter(ctx, stream, opts.IdleTimeout, opts.KeepaliveInterval)
	eventsAfterCancel := 0
	terminalSeen := false
	for {
		result := waiter.Next()
		if result.heartbeat {
			if _, err := c.Writer.Write([]byte(": keepalive\n\n")); err != nil {
				clientDisconnected = true
				log.Warn(ctx, "Failed to write SSE keepalive", log.Cause(err))
				return
			}
			c.Writer.Flush()
			continue
		}
		if result.ok {
			cur := result.event
			if orchestrator.IsTerminalStreamEvent(cur) {
				terminalSeen = true
			}
			c.SSEvent(cur.Type, cur.Data)
			log.Debug(ctx, "write stream event", log.Any("event", cur))
			c.Writer.Flush()
			continue
		}

		if result.err != nil {
			if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
				// Drain remaining buffered events so the request is marked
				// completed rather than canceled when the client disconnects
				// right after the terminal event.
				if buffered := waiter.DrainBuffered(); buffered.ok {
					c.SSEvent(buffered.event.Type, buffered.event.Data)
					c.Writer.Flush()
				}
				for stream.Next() {
					eventsAfterCancel++
					if eventsAfterCancel > maxStreamEventsAfterCancel {
						clientDisconnected = true
						log.Warn(ctx, "Stream still producing after cancellation, aborting drain",
							log.Int("events_after_cancel", eventsAfterCancel))
						return
					}
					cur := stream.Current()
					c.SSEvent(cur.Type, cur.Data)
					c.Writer.Flush()
				}
				if err := stream.Err(); err != nil && !errors.Is(err, context.Canceled) {
					log.Warn(ctx, "Stream error after client disconnected", log.Cause(err))
				}
				clientDisconnected = true
				log.Warn(ctx, "Context done, stopping stream", log.Cause(result.err))
				return
			}

			if errors.Is(result.err, ErrStreamIdleTimeout) {
				log.Warn(ctx, "Stream idle timeout, stopping stream", log.Duration("idle_timeout", opts.IdleTimeout), log.Cause(result.err))
			} else {
				log.Error(ctx, "Error in stream", log.Cause(result.err))
			}

			c.SSEvent("error", formatErr(ctx, orchestrator.ClassifyUpstreamTransportError(result.err)))
		} else if !terminalSeen {
			log.Error(ctx, "Stream ended without terminal event, reporting incomplete stream to client",
				log.Cause(orchestrator.ErrStreamIncomplete))
			c.SSEvent("error", formatErr(ctx, orchestrator.ClassifyUpstreamTransportError(orchestrator.ErrStreamIncomplete)))
		}

		c.Writer.Flush()
		return
	}
}

func writeSSEStreamWithHeartbeat(
	c *gin.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	formatErr StreamErrorFormatter,
	interval time.Duration,
	heartbeatFormat sseHeartbeatFormat,
) {
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

	setSSEHeaders(c)
	c.Writer.Flush()

	reader := newSSEStreamReader(ctx, stream)
	// The caller closes the stream after this function returns. Wait for the
	// reader first so Close cannot race with Next or Current.
	defer reader.Stop()

	timer := time.NewTimer(interval)
	defer timer.Stop()

	timerC := timer.C
	ctxDone := ctx.Done()
	eventsAfterCancel := 0
	terminalSeen := false
	heartbeatCount := 0

	for {
		select {
		case <-ctxDone:
			clientDisconnected = true
			ctxDone = nil
			stopTimer(timer)
			timerC = nil

		case result := <-reader.Results():
			if result.done {
				writeSSEStreamEnd(c, ctx, result.err, formatErr, terminalSeen, &clientDisconnected)
				return
			}

			if ctx.Err() != nil {
				eventsAfterCancel++
				if eventsAfterCancel > maxStreamEventsAfterCancel {
					clientDisconnected = true
					log.Warn(ctx, "Stream still producing after cancellation, aborting drain",
						log.Int("events_after_cancel", eventsAfterCancel))
					return
				}
			}

			cur := result.event
			if orchestrator.IsTerminalStreamEvent(cur) {
				terminalSeen = true
			}
			c.SSEvent(cur.Type, cur.Data)
			log.Debug(ctx, "write stream event", log.Any("event", cur))
			c.Writer.Flush()

			if timerC != nil {
				resetTimer(timer, interval)
			}

		case <-timerC:
			if err := writeSSEHeartbeat(c.Writer, heartbeatFormat); err != nil {
				clientDisconnected = true
				log.Warn(ctx, "Failed to write SSE heartbeat", log.Cause(err))
				return
			}

			heartbeatCount++
			log.Info(ctx, "SSE heartbeat sent",
				log.Int("heartbeat_count", heartbeatCount),
				log.String("heartbeat_format", sseHeartbeatFormatName(heartbeatFormat)),
				log.Duration("interval", interval),
			)

			c.Writer.Flush()
			timer.Reset(interval)
		}
	}
}

func writeSSEStreamEnd(
	c *gin.Context,
	ctx context.Context,
	streamErr error,
	formatErr StreamErrorFormatter,
	terminalSeen bool,
	clientDisconnected *bool,
) {
	switch {
	case streamErr != nil:
		if errors.Is(streamErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			*clientDisconnected = true

			if !errors.Is(streamErr, context.Canceled) {
				log.Warn(ctx, "Stream error after client disconnected", log.Cause(streamErr))
			}
		} else {
			log.Error(ctx, "Error in stream", log.Cause(streamErr))
			c.SSEvent("error", formatErr(ctx, orchestrator.ClassifyUpstreamTransportError(streamErr)))
		}
	case errors.Is(ctx.Err(), context.Canceled):
		*clientDisconnected = true
	case !terminalSeen:
		log.Error(ctx, "Stream ended without terminal event, reporting incomplete stream to client",
			log.Cause(orchestrator.ErrStreamIncomplete))
		c.SSEvent("error", formatErr(ctx, orchestrator.ClassifyUpstreamTransportError(orchestrator.ErrStreamIncomplete)))
	}

	c.Writer.Flush()
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func setSSEHeaders(c *gin.Context) {
	setSSEResponseHeaders(c.Writer.Header())
}

func setSSEResponseHeaders(header http.Header) {
	header.Set("Content-Type", sse.ContentType)
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
}

func writeSSEHeartbeat(writer io.Writer, format sseHeartbeatFormat) error {
	switch format {
	case sseHeartbeatOpenAI:
		_, err := io.WriteString(writer, ": keep-alive\n\n")
		return err
	case sseHeartbeatAnthropic:
		_, err := io.WriteString(writer, "event: ping\ndata: {\"type\":\"ping\"}\n\n")
		return err
	default:
		return errors.New("unsupported SSE heartbeat format")
	}
}

func sseHeartbeatFormatName(format sseHeartbeatFormat) string {
	switch format {
	case sseHeartbeatOpenAI:
		return "openai"
	case sseHeartbeatAnthropic:
		return "anthropic"
	default:
		return "unknown"
	}
}

// WriteBinaryStream writes raw bytes from stream events directly to the response body.
// The first chunk type is treated as the stream Content-Type when present.
func WriteBinaryStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	WriteBinaryStreamWithOptions(c, stream, StreamWriteOptions{})
}

func WriteBinaryStreamWithOptions(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions) {
	ctx := c.Request.Context()
	clientDisconnected := false
	headersWritten := false
	contentType := "application/octet-stream"

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	for {
		result := nextStreamEvent(ctx, stream, opts.IdleTimeout)
		if !result.ok {
			if result.err != nil {
				if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
					// Drain remaining buffered events so the request is marked
					// completed rather than canceled when the client disconnects
					// right after the terminal event.
					eventsAfterCancel := 0
					for stream.Next() {
						eventsAfterCancel++
						if eventsAfterCancel > maxStreamEventsAfterCancel {
							clientDisconnected = true
							log.Warn(ctx, "Binary stream still producing after cancellation, aborting drain",
								log.Int("events_after_cancel", eventsAfterCancel))
							return
						}
					}
					if err := stream.Err(); err != nil && !errors.Is(err, context.Canceled) {
						log.Warn(ctx, "Binary stream error after client disconnected", log.Cause(err))
					}
					clientDisconnected = true
					log.Warn(ctx, "Context done, stopping binary stream", log.Cause(result.err))
					return
				}

				if errors.Is(result.err, ErrStreamIdleTimeout) {
					log.Warn(ctx, "Stream idle timeout, stopping binary stream", log.Duration("idle_timeout", opts.IdleTimeout), log.Cause(result.err))
				} else {
					log.Error(ctx, "Error in binary stream", log.Cause(result.err))
				}

				if !headersWritten {
					failure := orchestrator.ClassifyUpstreamTransportError(result.err)
					c.JSON(streamErrorStatus(failure), FormatStreamError(ctx, failure))
					return
				}
			}

			c.Writer.Flush()
			return
		}

		cur := result.event
		if cur != nil && cur.Type == httpclient.BinaryStreamDoneEventType {
			continue
		}

		if cur == nil || len(cur.Data) == 0 {
			continue
		}

		if !headersWritten {
			if ct := strings.TrimSpace(cur.Type); ct != "" {
				contentType = ct
			}

			c.Header("Content-Type", contentType)
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Access-Control-Allow-Origin", "*")
			headersWritten = true
		}

		if _, err := c.Writer.Write(cur.Data); err != nil {
			clientDisconnected = true
			log.Warn(ctx, "Failed to write binary stream chunk", log.Cause(err))
			return
		}

		c.Writer.Flush()
	}
}

func streamErrorStatus(err error) int {
	var quotaErr *orchestrator.QuotaExhaustedError
	if errors.As(err, &quotaErr) {
		return http.StatusServiceUnavailable
	}

	var respErr *llm.ResponseError
	if errors.As(err, &respErr) && respErr.StatusCode != 0 {
		return respErr.StatusCode
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) && httpErr.StatusCode != 0 {
		return httpErr.StatusCode
	}

	return http.StatusInternalServerError
}

// FormatStreamError formats a stream error into an OpenAI-compatible JSON error object.
// When the error carries no upstream request id, the gateway's own request id from ctx
// is used so clients can always correlate an error event with the request log.
func FormatStreamError(ctx context.Context, err error) any {
	errType := "server_error"
	errCode := ""
	requestID := ""

	var quotaErr *orchestrator.QuotaExhaustedError
	if errors.As(err, &quotaErr) {
		return gin.H{
			"error": gin.H{
				"message": quotaErr.Error(),
				"type":    errTypeQuotaExhausted,
				"code":    errCodeQuotaExhausted,
			},
		}
	}

	var respErr *llm.ResponseError
	if errors.As(err, &respErr) {
		if respErr.Detail.Type != "" {
			errType = respErr.Detail.Type
		}

		errCode = respErr.Detail.Code
		requestID = respErr.Detail.RequestID

		return gin.H{
			"error": gin.H{
				"message": respErr.Detail.Message,
				"type":    errType,
				"code":    errCode,
			},
			"request_id": streamErrorRequestID(ctx, requestID),
		}
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) && len(httpErr.Body) > 0 {
		if t := gjson.GetBytes(httpErr.Body, "error.type"); t.Exists() && t.Type == gjson.String && t.String() != "" {
			errType = t.String()
		}

		if c := gjson.GetBytes(httpErr.Body, "error.code"); c.Exists() && c.Type == gjson.String && c.String() != "" {
			errCode = c.String()
		}

		if rid := gjson.GetBytes(httpErr.Body, "request_id"); rid.Exists() && rid.Type == gjson.String && rid.String() != "" {
			requestID = rid.String()
		}
	}

	return gin.H{
		"error": gin.H{
			"message": orchestrator.ExtractErrorMessage(err),
			"type":    errType,
			"code":    errCode,
		},
		"request_id": streamErrorRequestID(ctx, requestID),
	}
}

// streamErrorRequestID returns the upstream request id when present, otherwise the
// gateway request id stored in ctx (the value echoed in the AH-Request-Id header).
func streamErrorRequestID(ctx context.Context, requestID string) string {
	if requestID != "" || ctx == nil {
		return requestID
	}

	if id, ok := contexts.GetRequestID(ctx); ok {
		return id
	}

	return ""
}

func wrapQuotaExhaustedAsResponseError(err error) error {
	if err == nil {
		return nil
	}

	var quotaErr *orchestrator.QuotaExhaustedError
	if errors.As(err, &quotaErr) {
		return &llm.ResponseError{
			StatusCode: http.StatusServiceUnavailable,
			Detail: llm.ErrorDetail{
				Message: quotaErr.Error(),
				Type:    errTypeQuotaExhausted,
				Code:    errCodeQuotaExhausted,
			},
		}
	}

	return err
}
