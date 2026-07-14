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

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/internal/server/orchestrator"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/streams"
)

type timeoutSelectionProcessor struct {
	lastDeadlineSet bool
	lastDeadline    time.Time
	result          orchestrator.ChatCompletionResult
}

func (p *timeoutSelectionProcessor) Process(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
	deadline, ok := ctx.Deadline()
	p.lastDeadlineSet = ok
	p.lastDeadline = deadline

	return p.result, nil
}

type delayedTimeoutSelectionProcessor struct {
	delay  time.Duration
	result orchestrator.ChatCompletionResult
	err    error
}

func (p *delayedTimeoutSelectionProcessor) Process(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
	select {
	case <-time.After(p.delay):
		return p.result, p.err
	case <-ctx.Done():
		return orchestrator.ChatCompletionResult{}, ctx.Err()
	}
}

func TestChatCompletionWithRequestLoadsConfiguredKeepaliveWithoutUser(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	setupCtx := ent.NewContext(authz.WithTestBypass(t.Context()), client)
	require.NoError(t, systemService.SetStreamingSettings(setupCtx, &biz.StreamingSettings{
		HTTPStreamKeepaliveIntervalSeconds: 1,
	}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request = c.Request.WithContext(ent.NewContext(c.Request.Context(), client))

	handler := &ChatCompletionHandlers{
		ChatCompletionOrchestrator: &orchestrator.ChatCompletionOrchestrator{SystemService: systemService},
		Processor: &delayedTimeoutSelectionProcessor{
			delay: 1100 * time.Millisecond,
			result: orchestrator.ChatCompletionResult{
				ChatCompletionStream: streams.SliceStream([]*httpclient.StreamEvent{{Type: "message", Data: []byte(`{"ok":true}`)}}),
			},
		},
		StreamWriter:      WriteSSEStreamWithOptions,
		StreamIdleTimeout: time.Second,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{Body: []byte(`{"model":"test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)})

	require.Contains(t, w.Body.String(), ": keepalive\n\n")
	require.Contains(t, w.Body.String(), `{"ok":true}`)
}

func TestChatCompletionWithRequestEmitsKeepaliveBeforeProcessReturns(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	processor := &delayedTimeoutSelectionProcessor{
		delay: 25 * time.Millisecond,
		result: orchestrator.ChatCompletionResult{
			ChatCompletionStream: streams.SliceStream([]*httpclient.StreamEvent{{Type: "message", Data: []byte(`{"ok":true}`)}}),
		},
	}
	handler := &ChatCompletionHandlers{
		Processor:             processor,
		StreamWriter:          WriteSSEStreamWithOptions,
		StreamIdleTimeout:     time.Second,
		HTTPKeepaliveInterval: 5 * time.Millisecond,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{Body: []byte(`{"model":"test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)})

	require.Contains(t, w.Body.String(), ": keepalive\n\n")
	require.Contains(t, w.Body.String(), `{"ok":true}`)
}

func TestChatCompletionWithRequestKeepsGeminiJSONValidBeforeProcessReturns(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/test:streamGenerateContent", nil)

	handler := &ChatCompletionHandlers{
		Processor: &delayedTimeoutSelectionProcessor{
			delay: 20 * time.Millisecond,
			result: orchestrator.ChatCompletionResult{
				ChatCompletionStream: streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte(`{"step":1}`)}}),
			},
		},
		StreamWriter:          WriteGeminiStreamWithOptions,
		StreamIdleTimeout:     time.Second,
		HTTPKeepaliveInterval: 5 * time.Millisecond,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{
		Path: "/v1beta/models/test:streamGenerateContent",
		Body: []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`),
	})

	var body []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, float64(1), body[0]["step"])
}

func TestChatCompletionWithRequestWritesValidGeminiErrorAfterKeepalive(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/test:streamGenerateContent", nil)

	handler := &ChatCompletionHandlers{
		Processor: &delayedTimeoutSelectionProcessor{
			delay: 20 * time.Millisecond,
			err:   errors.New("process failed"),
		},
		StreamWriter:          WriteGeminiStreamWithOptions,
		HTTPKeepaliveInterval: 5 * time.Millisecond,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{
		Path: "/v1beta/models/test:streamGenerateContent",
		Body: []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`),
	})

	var body []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotEmpty(t, body[0]["error"])
}

func TestChatCompletionWithRequestDoesNotInjectKeepaliveIntoBinaryStream(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	handler := &ChatCompletionHandlers{
		Processor: &delayedTimeoutSelectionProcessor{
			delay: 20 * time.Millisecond,
			result: orchestrator.ChatCompletionResult{
				ChatCompletionStream: streams.SliceStream([]*httpclient.StreamEvent{{Type: "audio/mpeg", Data: []byte{0x01, 0x02, 0x03}}}),
			},
		},
		StreamWriter:          WriteBinaryStreamWithOptions,
		StreamIdleTimeout:     time.Second,
		HTTPKeepaliveInterval: 5 * time.Millisecond,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{Body: []byte(`{"model":"tts","input":"hi","stream_format":"mp3"}`)})

	require.Equal(t, []byte{0x01, 0x02, 0x03}, w.Body.Bytes())
}

func TestChatCompletionWithRequestWritesStreamErrorAfterKeepaliveCommitsResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	handler := &ChatCompletionHandlers{
		Processor: &delayedTimeoutSelectionProcessor{
			delay: 20 * time.Millisecond,
			err:   errors.New("process failed"),
		},
		StreamWriter:          WriteSSEStreamWithOptions,
		HTTPKeepaliveInterval: 5 * time.Millisecond,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{Body: []byte(`{"model":"test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)})

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), ": keepalive\n\n")
	require.Contains(t, w.Body.String(), "event:error")
}

func TestChatCompletionWithRequestKeepsHTTPStatusBeforeFirstKeepalive(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	handler := &ChatCompletionHandlers{
		Processor: &delayedTimeoutSelectionProcessor{
			delay: time.Millisecond,
			err:   errors.New("process failed"),
		},
		StreamWriter:          WriteSSEStreamWithOptions,
		HTTPKeepaliveInterval: time.Second,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{Body: []byte(`{"model":"test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)})

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.NotContains(t, w.Body.String(), ": keepalive")
}

func TestChatCompletionWithRequestAppliesTotalDeadlineToNonStreamingRequest(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	processor := &timeoutSelectionProcessor{}
	handler := &ChatCompletionHandlers{
		Processor:         processor,
		RequestTimeout:    20 * time.Millisecond,
		StreamWriter:      func(*gin.Context, streams.Stream[*httpclient.StreamEvent], StreamWriteOptions) {},
		StreamIdleTimeout: time.Second,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{Body: []byte(`{"model":"test","stream":false,"messages":[{"role":"user","content":"hi"}]}`)})

	require.True(t, processor.lastDeadlineSet, "non-streaming request should get total deadline")
	require.WithinDuration(t, time.Now().Add(20*time.Millisecond), processor.lastDeadline, 100*time.Millisecond)
}

func TestChatCompletionWithRequestDoesNotApplyTotalDeadlineToStreamingRequest(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	processor := &timeoutSelectionProcessor{}
	handler := &ChatCompletionHandlers{
		Processor:         processor,
		RequestTimeout:    20 * time.Millisecond,
		StreamWriter:      func(*gin.Context, streams.Stream[*httpclient.StreamEvent], StreamWriteOptions) {},
		StreamIdleTimeout: time.Second,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{Body: []byte(`{"model":"test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)})

	require.False(t, processor.lastDeadlineSet, "streaming request should not get total LLMRequestTimeout deadline")
}

func TestChatCompletionWithRequestDoesNotApplyTotalDeadlineToGeminiStreamingPath(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:streamGenerateContent", nil)

	processor := &timeoutSelectionProcessor{}
	handler := &ChatCompletionHandlers{
		Processor:         processor,
		RequestTimeout:    20 * time.Millisecond,
		StreamWriter:      func(*gin.Context, streams.Stream[*httpclient.StreamEvent], StreamWriteOptions) {},
		StreamIdleTimeout: time.Second,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{
		Path: "/v1beta/models/gemini-2.5-flash:streamGenerateContent",
		Body: []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`),
	})

	require.False(t, processor.lastDeadlineSet, "Gemini streaming path should not get total LLMRequestTimeout deadline")
}

func TestChatCompletionWithRequestDoesNotApplyTotalDeadlineToSpeechStreamingFormat(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	processor := &timeoutSelectionProcessor{}
	handler := &ChatCompletionHandlers{
		Processor:         processor,
		RequestTimeout:    20 * time.Millisecond,
		StreamWriter:      func(*gin.Context, streams.Stream[*httpclient.StreamEvent], StreamWriteOptions) {},
		StreamIdleTimeout: time.Second,
	}

	handler.ChatCompletionWithRequest(c, &httpclient.Request{Body: []byte(`{"model":"tts","input":"hi","stream_format":"mp3"}`)})

	require.False(t, processor.lastDeadlineSet, "speech streaming format should not get total LLMRequestTimeout deadline")
}
