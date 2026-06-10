package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/streams"
)

var ErrStreamIdleTimeout = errors.New("stream idle timeout")

type StreamWriteOptions struct {
	IdleTimeout time.Duration
}

type TimeoutConfig struct {
	LLMRequestTimeout    time.Duration
	LLMStreamIdleTimeout time.Duration
}

func (handlers *ChatCompletionHandlers) WithTimeouts(config TimeoutConfig) *ChatCompletionHandlers {
	if handlers == nil {
		return nil
	}

	handlers.RequestTimeout = config.LLMRequestTimeout
	handlers.StreamIdleTimeout = config.LLMStreamIdleTimeout

	return handlers
}

type streamNextResult struct {
	event *httpclient.StreamEvent
	ok    bool
	err   error
}

func nextStreamEvent(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent], idleTimeout time.Duration) streamNextResult {
	if idleTimeout <= 0 {
		if stream.Next() {
			return streamNextResult{event: stream.Current(), ok: true}
		}

		return streamNextResult{err: stream.Err()}
	}

	resultCh := make(chan streamNextResult, 1)
	go func() {
		if stream.Next() {
			resultCh <- streamNextResult{event: stream.Current(), ok: true}
			return
		}

		resultCh <- streamNextResult{err: stream.Err()}
	}()

	timer := time.NewTimer(idleTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		_ = stream.Close()
		return streamNextResult{err: ctx.Err()}
	case <-timer.C:
		_ = stream.Close()
		return streamNextResult{err: fmt.Errorf("%w after %s", ErrStreamIdleTimeout, idleTimeout)}
	case result := <-resultCh:
		return result
	}
}
