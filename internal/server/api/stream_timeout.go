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
	// Cancel interrupts the context used to create the upstream stream. Writers
	// join pending reads before the caller closes mutable stream wrappers.
	Cancel          context.CancelFunc
	heartbeatFormat sseHeartbeatFormat
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
	cancel            context.CancelFunc
}

func newStreamEventWaiter(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	opts StreamWriteOptions,
) *streamEventWaiter {
	waiter := &streamEventWaiter{
		ctx:               ctx,
		stream:            stream,
		idleTimeout:       opts.IdleTimeout,
		keepaliveInterval: opts.KeepaliveInterval,
		resultCh:          make(chan streamNextResult, 1),
		cancel:            opts.Cancel,
	}
	if opts.IdleTimeout > 0 {
		waiter.idleTimer = time.NewTimer(opts.IdleTimeout)
	}
	if opts.KeepaliveInterval > 0 {
		waiter.keepaliveTimer = time.NewTimer(opts.KeepaliveInterval)
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

	// Hand off any pending read before returning cancellation. Stream writers
	// may drain or close next; neither operation may race with Next/Current.
	select {
	case <-w.ctx.Done():
		return w.canceled()
	default:
	}

	w.startRead()
	select {
	case <-w.ctx.Done():
		return w.canceled()
	case <-timerChannel(w.idleTimer):
		w.done = true
		if w.cancel != nil {
			w.cancel()
		} else {
			// Standalone writers without an upstream cancel function require a
			// transport stream whose Close unblocks Next.
			_ = w.stream.Close()
		}
		w.joinRead()
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

func (w *streamEventWaiter) joinRead() {
	if w.reading {
		<-w.resultCh
		w.reading = false
	}
}

func (w *streamEventWaiter) Stop() {
	if w.idleTimer != nil {
		w.idleTimer.Stop()
	}
	if w.keepaliveTimer != nil {
		w.keepaliveTimer.Stop()
	}
	if w.reading && w.cancel != nil {
		w.cancel()
	}
	w.joinRead()
}

func (w *streamEventWaiter) canceled() streamNextResult {
	if w.reading {
		result := <-w.resultCh
		w.reading = false
		// Preserve the final buffered event for the cancellation drain.
		w.resultCh <- result
	}
	w.done = true
	return streamNextResult{err: w.ctx.Err()}
}

// DrainBuffered returns any event already read by the background goroutine
// but not yet consumed by Next(). Used after context cancellation to avoid
// losing events that were read before the cancel was observed.
func (w *streamEventWaiter) DrainBuffered() streamNextResult {
	select {
	case result := <-w.resultCh:
		return result
	default:
		return streamNextResult{}
	}
}

func nextStreamEvent(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent], idleTimeout time.Duration) streamNextResult {
	waiter := newStreamEventWaiter(ctx, stream, StreamWriteOptions{IdleTimeout: idleTimeout})
	defer waiter.Stop()
	return waiter.Next()
}
