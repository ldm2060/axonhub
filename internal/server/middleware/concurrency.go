package middleware

import (
	"context"
	"net/http"
	"strconv"
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
	if c.limit <= 0 {
		return 0, true
	}
	if c.users[key] >= c.limit {
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
		return nil, &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Status:     http.StatusText(http.StatusTooManyRequests),
			Body:       []byte(`{"error":{"message":"too many concurrent requests for this user, retry shortly","type":"rate_limit_error","code":"concurrency_limit_exceeded"}}`),
			Headers:    http.Header{"Retry-After": {"1"}},
		}
	}
	if limit == 0 {
		return func() {}, nil
	}
	return sync.OnceFunc(func() { r.config.release(r.key) }), nil
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
		if cfg == nil {
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
		if c.Request.Method == http.MethodGet && websocket.IsWebSocketUpgrade(c.Request) {
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
// attributed to the key owner, falling back to the key itself when the owner
// could not be resolved.
func concurrencyKey(c *gin.Context) (string, bool) {
	if user, ok := contexts.GetActingUser(c.Request.Context()); ok && user != nil {
		return "user:" + strconv.Itoa(user.ID), true
	}

	if apiKey, ok := contexts.GetAPIKey(c.Request.Context()); ok && apiKey != nil {
		return "api_key:" + strconv.Itoa(apiKey.ID), true
	}

	return "", false
}
