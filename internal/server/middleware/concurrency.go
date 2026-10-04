package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

// ConcurrencyLimitConfig holds a reloadable per-user cap on concurrent LLM
// requests. HTTP requests and active WebSocket messages share the same budget.
type ConcurrencyLimitConfig struct {
	mu       sync.Mutex
	limit    int
	users    map[string]int
	rejected int64
}

type webSocketConcurrencyKey struct{}

type requestConcurrency struct {
	config *ConcurrencyLimitConfig
	key    string
	path   string
	method string
}

// NewConcurrencyLimitConfig creates the limiter. A limit <= 0 disables it.
func NewConcurrencyLimitConfig(limit int) *ConcurrencyLimitConfig {
	cfg := &ConcurrencyLimitConfig{}
	cfg.Apply(limit)

	return cfg
}

// Apply updates the cap without forgetting requests that are still in flight.
// Existing WebSocket connections consult this cap for each new message.
func (c *ConcurrencyLimitConfig) Apply(limit int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.limit = max(limit, 0)
}

// Stats reports the configured per-user limit, the number of tracked users and
// the total number of rejections.
func (c *ConcurrencyLimitConfig) Stats() (limit, trackedUsers, rejected int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return int64(c.limit), int64(len(c.users)), c.rejected
}

func (c *ConcurrencyLimitConfig) acquire(key string) (limit int, admitted bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Track active work even while the cap is disabled, so enabling it cannot
	// admit a second budget on top of requests that are already running.
	if c.limit > 0 && c.users[key] >= c.limit {
		c.rejected++
		return c.limit, false
	}
	if c.users == nil {
		c.users = make(map[string]int)
	}
	c.users[key]++
	return c.limit, true
}

func (c *ConcurrencyLimitConfig) release(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.users[key] <= 1 {
		delete(c.users, key)
	} else {
		c.users[key]--
	}
}

func (r *requestConcurrency) acquire(ctx context.Context) (func(), *httpclient.Error) {
	limit, admitted := r.config.acquire(r.key)
	if !admitted {
		log.Warn(ctx, "per-user concurrency limit reached, rejecting request",
			log.Int("limit", limit), log.String("user", r.key),
			log.String("path", r.path), log.String("method", r.method))
		return nil, concurrencyLimitError(r.path)
	}
	return sync.OnceFunc(func() { r.config.release(r.key) }), nil
}

func concurrencyLimitError(path string) *httpclient.Error {
	body := `{"error":{"message":"too many concurrent requests for this user, retry shortly","type":"rate_limit_error","code":"concurrency_limit_exceeded"}}`
	switch {
	case strings.HasSuffix(path, "/messages"):
		body = `{"type":"error","error":{"message":"too many concurrent requests for this user, retry shortly","type":"rate_limit_error","code":"concurrency_limit_exceeded"}}`
	case strings.Contains(path, "/gemini/"), strings.Contains(path, "/v1beta/"):
		body = `{"error":{"code":429,"message":"too many concurrent requests for this user, retry shortly","status":"RESOURCE_EXHAUSTED"}}`
	}
	return &httpclient.Error{
		StatusCode: http.StatusTooManyRequests,
		Status:     http.StatusText(http.StatusTooManyRequests),
		Body:       []byte(body),
		Headers:    http.Header{"Retry-After": {"1"}},
	}
}

// AcquireWebSocketConcurrency reserves a user slot for one generation on an
// upgraded connection. The caller must release it after writing/closing the
// entire response stream, including error and cancellation paths.
func AcquireWebSocketConcurrency(ctx context.Context) (func(), *httpclient.Error) {
	if request, ok := ctx.Value(webSocketConcurrencyKey{}).(*requestConcurrency); ok {
		return request.acquire(ctx)
	}
	return func() {}, nil
}

// WithConcurrencyLimit reserves slots for HTTP requests before reading their
// bodies. WebSocket upgrades only attach the shared limiter: their handlers
// acquire a slot per generation, so idle connections consume no user budget.
func WithConcurrencyLimit(cfg *ConcurrencyLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		isWebSocket := c.Request.Method == http.MethodGet && websocket.IsWebSocketUpgrade(c.Request)
		if cfg == nil || (!isWebSocket && (c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodDelete || c.Request.Method == http.MethodOptions)) {
			// Model discovery, task polling and cancellation do not start a
			// generation, and must remain available while its budget is full.
			c.Next()
			return
		}

		key, ok := concurrencyKey(c)
		if !ok {
			// No resolved principal: nothing to attribute the request to, so
			// the cap cannot be applied without blocking unrelated users.
			c.Next()
			return
		}

		request := &requestConcurrency{config: cfg, key: key, path: c.Request.URL.Path, method: c.Request.Method}
		if isWebSocket {
			ctx := context.WithValue(c.Request.Context(), webSocketConcurrencyKey{}, request)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		release, err := request.acquire(c.Request.Context())
		if err != nil {
			c.Header("Retry-After", err.Headers.Get("Retry-After"))
			c.Data(err.StatusCode, "application/json", err.Body)
			c.Abort()
			return
		}
		defer release()

		c.Next()
	}
}

// concurrencyKey identifies the user a request acts as. API-key requests are
// attributed to the key's owner ID even when loading the user entity failed.
// Ownerless keys have independent key-level budgets.
func concurrencyKey(c *gin.Context) (string, bool) {
	if user, ok := contexts.GetActingUser(c.Request.Context()); ok && user != nil {
		return "user:" + strconv.Itoa(user.ID), true
	}

	if apiKey, ok := contexts.GetAPIKey(c.Request.Context()); ok && apiKey != nil {
		if apiKey.UserID > 0 {
			return "user:" + strconv.Itoa(apiKey.UserID), true
		}
		return "api_key:" + strconv.Itoa(apiKey.ID), true
	}

	return "", false
}
