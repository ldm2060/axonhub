package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/log"
)

// ConcurrencyLimitConfig holds a reloadable per-user cap on concurrent LLM
// requests. The snapshot is swapped atomically so a config reload takes effect
// for new requests without rebuilding routes.
type ConcurrencyLimitConfig struct {
	ptr atomic.Pointer[concurrencyLimitState]
}

type concurrencyLimitState struct {
	limit int

	mu       sync.Mutex
	users    map[string]*atomic.Int64
	rejected atomic.Int64
}

// NewConcurrencyLimitConfig creates the limiter. A limit <= 0 disables it.
func NewConcurrencyLimitConfig(limit int) *ConcurrencyLimitConfig {
	cfg := &ConcurrencyLimitConfig{}
	cfg.Apply(limit)

	return cfg
}

// Apply swaps in a new limit. Requests already admitted keep their slot and
// still decrement the snapshot they acquired from, so shrinking the limit never
// orphans an in-flight request.
func (c *ConcurrencyLimitConfig) Apply(limit int) {
	if limit < 0 {
		limit = 0
	}

	c.ptr.Store(&concurrencyLimitState{limit: limit, users: map[string]*atomic.Int64{}})
}

// Stats reports the configured per-user limit, the number of tracked users and
// the total number of rejections.
func (c *ConcurrencyLimitConfig) Stats() (limit, trackedUsers, rejected int64) {
	state := c.ptr.Load()
	if state == nil {
		return 0, 0, 0
	}

	state.mu.Lock()
	tracked := int64(len(state.users))
	state.mu.Unlock()

	return int64(state.limit), tracked, state.rejected.Load()
}

func (s *concurrencyLimitState) acquire(key string) bool {
	s.mu.Lock()
	counter, ok := s.users[key]
	if !ok {
		counter = &atomic.Int64{}
		s.users[key] = counter
	}
	s.mu.Unlock()

	for {
		current := counter.Load()
		if int(current) >= s.limit {
			return false
		}

		if counter.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func (s *concurrencyLimitState) release(key string) {
	s.mu.Lock()
	counter := s.users[key]
	s.mu.Unlock()

	if counter == nil {
		return
	}

	// Drop the entry once the user goes idle so the map stays bounded by the
	// number of concurrently active users rather than every user ever seen.
	if counter.Add(-1) == 0 {
		s.mu.Lock()
		if counter.Load() == 0 {
			delete(s.users, key)
		}
		s.mu.Unlock()
	}
}

// WithConcurrencyLimit caps how many requests each individual user may have in
// flight. Exceeding the cap returns 429 with Retry-After. A limit of 0 leaves
// the request path untouched.
func WithConcurrencyLimit(cfg *ConcurrencyLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		state := cfg.ptr.Load()
		if state == nil || state.limit <= 0 {
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

		// Reserve the slot before the handler runs so the cap covers the whole
		// request lifetime, including the body read that dominates memory use.
		if !state.acquire(key) {
			state.rejected.Add(1)

			log.Warn(
				c.Request.Context(),
				"per-user concurrency limit reached, rejecting request",
				log.Int("limit", state.limit),
				log.String("user", key),
				log.String("path", c.Request.URL.Path),
				log.String("method", c.Request.Method),
			)

			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"message": "too many concurrent requests for this user, retry shortly",
					"type":    "rate_limit_error",
				},
			})

			return
		}

		defer state.release(key)

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
