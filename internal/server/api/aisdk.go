package api

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/internal/server/orchestrator"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/streams"
	"github.com/ldm2060/axonhub/llm/transformer/aisdk"
)

type AiSdkHandlersParams struct {
	fx.In

	ChannelService              *biz.ChannelService
	ModelService                *biz.ModelService
	DefaultSelector             *orchestrator.DefaultSelector
	RequestService              *biz.RequestService
	SystemService               *biz.SystemService
	UsageLogService             *biz.UsageLogService
	PromptService               *biz.PromptService
	PromptProtectionRuleService *biz.PromptProtectionRuleService
	QuotaService                *biz.QuotaService
	HttpClient                  *httpclient.HttpClient
	LiveStreamRegistry          *biz.LiveStreamRegistry
	ChannelLimiterManager       *orchestrator.ChannelLimiterManager
	ProviderQuotaStatusProvider orchestrator.ProviderQuotaStatusProvider
	TimeoutConfig               TimeoutConfig
}

type AiSDKHandlers struct {
	ChatCompletionHandler *ChatCompletionHandlers
}

func NewAiSDKHandlers(params AiSdkHandlersParams) *AiSDKHandlers {
	return &AiSDKHandlers{
		ChatCompletionHandler: (&ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				aisdk.NewDataStreamTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
				params.ChannelLimiterManager,
				params.ProviderQuotaStatusProvider,
			),
			StreamWriter: WriteJSONStreamWithOptions,
		}).WithTimeouts(params.TimeoutConfig),
	}
}

func (handlers *AiSDKHandlers) ChatCompletion(c *gin.Context) {
	handlers.ChatCompletionHandler.ChatCompletion(c)
}

// WriteJSONStream writes stream events as plain JSON text stream.
func WriteJSONStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	WriteJSONStreamWithOptions(c, stream, StreamWriteOptions{})
}

func WriteJSONStreamWithOptions(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], opts StreamWriteOptions) {
	ctx := c.Request.Context()
	clientDisconnected := false

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	// Set text stream headers
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Vercel-AI-Data-Stream", "v1")

	for {
		result := nextStreamEvent(ctx, stream, opts.IdleTimeout)
		if result.ok {
			cur := result.event
			_, _ = c.Writer.Write(cur.Data)
			log.Debug(ctx, "write stream event", log.Any("event", cur))
			c.Writer.Flush()
			continue
		}

		if result.err != nil {
			if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
				clientDisconnected = true
				log.Warn(ctx, "Client disconnected, stop streaming", log.Cause(result.err))
				return
			}

			if errors.Is(result.err, ErrStreamIdleTimeout) {
				log.Warn(ctx, "Stream idle timeout, stopping AI SDK stream", log.Duration("idle_timeout", opts.IdleTimeout), log.Cause(result.err))
			} else {
				log.Error(ctx, "Error in stream", log.Cause(result.err))
			}

			_, _ = c.Writer.Write([]byte("3:" + `"` + result.err.Error() + `"` + "\n"))
		}

		return
	}
}
