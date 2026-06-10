package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

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
