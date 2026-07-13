package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/streams"
)

var ErrStreamIdleTimeout = errors.New("stream idle timeout")

type StreamWriteOptions struct {
	IdleTimeout              time.Duration
	KeepaliveInterval        time.Duration
	ResponseAlreadyCommitted bool
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
	event     *httpclient.StreamEvent
	ok        bool
	heartbeat bool
	err       error
}

type streamEventWaiter struct {
	ctx               context.Context
	stream            streams.Stream[*httpclient.StreamEvent]
	idleTimeout       time.Duration
	keepaliveInterval time.Duration
	resultCh          chan streamNextResult
	idleTimer         *time.Timer
	keepaliveTimer    *time.Timer
	reading           bool
	done              bool
}

func newStreamEventWaiter(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	idleTimeout time.Duration,
	keepaliveInterval time.Duration,
) *streamEventWaiter {
	waiter := &streamEventWaiter{
		ctx:               ctx,
		stream:            stream,
		idleTimeout:       idleTimeout,
		keepaliveInterval: keepaliveInterval,
		resultCh:          make(chan streamNextResult, 1),
	}
	if idleTimeout > 0 {
		waiter.idleTimer = time.NewTimer(idleTimeout)
	}
	if keepaliveInterval > 0 {
		waiter.keepaliveTimer = time.NewTimer(keepaliveInterval)
	}

	return waiter
}

func timerChannel(timer *time.Timer) <-chan time.Time {
	if timer == nil {
		return nil
	}

	return timer.C
}

func resetTimer(timer *time.Timer, interval time.Duration) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(interval)
}

func (w *streamEventWaiter) startRead() {
	if w.reading || w.done {
		return
	}
	w.reading = true
	go func() {
		defer func() {
			if cause := recover(); cause != nil {
				err := fmt.Errorf("stream read panic: %v", cause)
				log.Warn(w.ctx, "Stream read panic recovered", log.Any("panic", cause))
				w.resultCh <- streamNextResult{err: err}
			}
		}()

		if w.stream.Next() {
			w.resultCh <- streamNextResult{event: w.stream.Current(), ok: true}
			return
		}

		w.resultCh <- streamNextResult{err: w.stream.Err()}
	}()
}

func (w *streamEventWaiter) Next() streamNextResult {
	if w.done {
		return streamNextResult{}
	}

	w.startRead()
	select {
	case <-w.ctx.Done():
		w.done = true
		_ = w.stream.Close()
		return streamNextResult{err: w.ctx.Err()}
	case <-timerChannel(w.idleTimer):
		w.done = true
		_ = w.stream.Close()
		return streamNextResult{err: fmt.Errorf("%w after %s", ErrStreamIdleTimeout, w.idleTimeout)}
	case <-timerChannel(w.keepaliveTimer):
		resetTimer(w.keepaliveTimer, w.keepaliveInterval)
		return streamNextResult{heartbeat: true}
	case result := <-w.resultCh:
		w.reading = false
		if result.ok {
			resetTimer(w.idleTimer, w.idleTimeout)
			resetTimer(w.keepaliveTimer, w.keepaliveInterval)
		} else {
			w.done = true
		}
		return result
	}
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
