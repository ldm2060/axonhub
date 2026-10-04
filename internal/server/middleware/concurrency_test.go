package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newConcurrencyRouter(cfg *ConcurrencyLimitConfig, release chan struct{}, entered *sync.Map) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		// Stand in for WithAPIKeyConfig: the limiter runs after auth in the
		// real chain, so the acting user is already in context by then.
		user := &ent.User{ID: 1}
		if h := c.GetHeader("X-Test-User"); h != "" {
			user = &ent.User{ID: 2}
		}

		c.Request = c.Request.WithContext(contexts.WithPrincipalUser(c.Request.Context(), user))
	})
	r.Use(WithConcurrencyLimit(cfg))
	r.POST("/v1/responses", func(c *gin.Context) {
		entered.Store(c.GetHeader("X-Test-User"), true)
		<-release
		c.Status(http.StatusOK)
	})

	return r
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for condition")
		}

		time.Sleep(time.Millisecond)
	}
}

func TestWithConcurrencyLimit_DisabledByDefault(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(0)
	release := make(chan struct{})
	var entered sync.Map

	router := newConcurrencyRouter(cfg, release, &entered)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}

	// All eight share one user, so a disabled limiter must admit every one.
	waitFor(t, func() bool {
		count := 0
		entered.Range(func(_, _ any) bool { count++; return true })

		return count == 1
	})
	close(release)
	wg.Wait()

	limit, tracked, rejected := cfg.Stats()
	assert.Equal(t, int64(0), limit)
	assert.Equal(t, int64(0), tracked, "completed requests must leave no tracked users")
	assert.Equal(t, int64(0), rejected)
}

func TestWithConcurrencyLimit_PerUserBudget(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(2)
	release := make(chan struct{})
	var entered sync.Map

	router := newConcurrencyRouter(cfg, release, &entered)

	// Two users, three requests each: every user gets its own budget, so 2 of
	// each user's 3 requests must be rejected rather than a global 2 of 6.
	var wg sync.WaitGroup
	for _, user := range []string{"", "other"} {
		for range 3 {
			wg.Add(1)

			go func(user string) {
				defer wg.Done()

				req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				if user != "" {
					req.Header.Set("X-Test-User", user)
				}

				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
			}(user)
		}
	}

	waitFor(t, func() bool {
		_, tracked, rejected := cfg.Stats()

		return tracked == 2 && rejected == 2
	})

	close(release)
	wg.Wait()

	limit, tracked, rejected := cfg.Stats()
	assert.Equal(t, int64(2), limit)
	assert.Equal(t, int64(0), tracked, "idle entries must be released")
	assert.Equal(t, int64(2), rejected)
}

func TestWithConcurrencyLimit_RejectsBeyondCap(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(2)
	release := make(chan struct{})
	var entered sync.Map

	router := newConcurrencyRouter(cfg, release, &entered)

	const total = 6

	var wg sync.WaitGroup
	for range total {
		wg.Go(func() {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))

			if rec.Code == http.StatusTooManyRequests {
				assert.Equal(t, "1", rec.Header().Get("Retry-After"))
			}
		})
	}

	// Hold the admitted requests until every other request has been rejected,
	// otherwise a freed slot would let a queued request back in and make the
	// count depend on scheduling.
	waitFor(t, func() bool {
		_, _, rejected := cfg.Stats()

		return rejected == total-2
	})

	close(release)
	wg.Wait()

	_, tracked, rejected := cfg.Stats()
	assert.Equal(t, int64(0), tracked)
	assert.Equal(t, int64(total-2), rejected)
}

func TestWithConcurrencyLimit_ApplyIsReloadable(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(1)
	require.Equal(t, int64(1), mustLimit(cfg))

	cfg.Apply(0)
	assert.Equal(t, int64(0), mustLimit(cfg), "0 must disable the limiter")

	cfg.Apply(-5)
	assert.Equal(t, int64(0), mustLimit(cfg), "negative values are clamped to disabled")

	cfg.Apply(7)
	assert.Equal(t, int64(7), mustLimit(cfg))
}

func mustLimit(cfg *ConcurrencyLimitConfig) int64 {
	limit, _, _ := cfg.Stats()

	return limit
}

func TestConcurrencyLimitApplyKeepsInFlightRequests(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(2)
	for range 2 {
		_, admitted := cfg.acquire("user:1")
		require.True(t, admitted)
	}
	for _, limit := range []int{2, 1, 2} {
		cfg.Apply(limit)
		_, admitted := cfg.acquire("user:1")
		require.False(t, admitted, "reloading must retain the active requests")
	}
	cfg.Apply(3)
	_, admitted := cfg.acquire("user:1")
	require.True(t, admitted)
	cfg.Apply(0)
	cfg.Apply(1)
	for range 3 {
		_, admitted = cfg.acquire("user:1")
		require.False(t, admitted)
		cfg.release("user:1")
	}
	_, tracked, _ := cfg.Stats()
	require.Zero(t, tracked)
	_, admitted = cfg.acquire("user:1")
	require.True(t, admitted)
	cfg.release("user:1")
}

func TestConcurrencyLimitConcurrentAcquireAndIdleCleanup(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(1)
	var active atomic.Int64
	var exceeded atomic.Bool
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("panic during concurrent slot acquisition: %v", recovered)
					exceeded.Store(true)
				}
			}()
			for range 1000 {
				if _, admitted := cfg.acquire("user:1"); !admitted {
					runtime.Gosched()
					continue
				}
				if active.Add(1) > 1 {
					exceeded.Store(true)
				}
				runtime.Gosched()
				active.Add(-1)
				cfg.release("user:1")
			}
		})
	}
	wg.Wait()
	require.False(t, exceeded.Load(), "idle cleanup must not detach an active counter")
	_, tracked, _ := cfg.Stats()
	require.Zero(t, tracked)
}

func TestConcurrencyLimitPOSTCannotBypassWithUpgradeHeaders(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(1)
	_, admitted := cfg.acquire("user:1")
	require.True(t, admitted)
	defer cfg.release("user:1")

	router := newConcurrencyRouter(cfg, nil, new(sync.Map))
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestConcurrencySlotReleaseIsIdempotent(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(2)
	request := &requestConcurrency{config: cfg, key: "user:1"}
	first, err := request.acquire(context.Background())
	require.Nil(t, err)
	second, err := request.acquire(context.Background())
	require.Nil(t, err)
	first()
	first()
	_, tracked, _ := cfg.Stats()
	require.Equal(t, int64(1), tracked, "duplicate release must not release another request")
	second()
	_, tracked, _ = cfg.Stats()
	require.Zero(t, tracked)
}

func TestConcurrencyLimitEnablingCountsRequestsAdmittedWhileDisabled(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(0)
	request := &requestConcurrency{config: cfg, key: "user:1"}
	release, err := request.acquire(t.Context())
	require.Nil(t, err)
	cfg.Apply(1)
	_, limitErr := request.acquire(t.Context())
	require.NotNil(t, limitErr)
	require.Equal(t, http.StatusTooManyRequests, limitErr.StatusCode)
	release()
	release, err = request.acquire(t.Context())
	require.Nil(t, err)
	release()
}

func TestConcurrencyKeyRetainsOwnerAcrossKeysAndEntryPoints(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"jwt", contexts.WithUser(t.Context(), &ent.User{ID: 7}), "user:7"},
		{"personal key", contexts.WithPrincipalUser(t.Context(), &ent.User{ID: 7}), "user:7"},
		{"first key without loaded user", contexts.WithAPIKey(t.Context(), &ent.APIKey{ID: 1, UserID: 7}), "user:7"},
		{"second key without loaded user", contexts.WithAPIKey(t.Context(), &ent.APIKey{ID: 2, UserID: 7}), "user:7"},
		{"ownerless key", contexts.WithAPIKey(t.Context(), &ent.APIKey{ID: 3}), "api_key:3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequestWithContext(tc.ctx, http.MethodPost, "/v1/messages", nil)
			key, ok := concurrencyKey(c)
			require.True(t, ok)
			require.Equal(t, tc.want, key)
		})
	}
}

func TestConcurrencyLimitKeepsDiscoveryAndCancellationAvailable(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(1)
	_, admitted := cfg.acquire("user:1")
	require.True(t, admitted)
	defer cfg.release("user:1")
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithPrincipalUser(c.Request.Context(), &ent.User{ID: 1}))
	}, WithConcurrencyLimit(cfg))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/models"},
		{http.MethodGet, "/anthropic/v1/models"},
		{http.MethodGet, "/v1beta/models"},
		{http.MethodGet, "/v1/videos/job"},
		{http.MethodDelete, "/v1/videos/job"},
		{http.MethodGet, "/doubao/v3/contents/generations/tasks/job"},
		{http.MethodDelete, "/doubao/v3/contents/generations/tasks/job"},
	} {
		router.Handle(tc.method, tc.path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, http.StatusNoContent, recorder.Code, "%s %s", tc.method, tc.path)
	}
}

func TestConcurrencyLimitUsesNativeProtocolErrors(t *testing.T) {
	cfg := NewConcurrencyLimitConfig(1)
	_, admitted := cfg.acquire("user:1")
	require.True(t, admitted)
	defer cfg.release("user:1")
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithPrincipalUser(c.Request.Context(), &ent.User{ID: 1}))
	}, WithConcurrencyLimit(cfg))
	for _, tc := range []struct{ path, field, value string }{
		{"/v1/chat/completions", "error.code", "concurrency_limit_exceeded"},
		{"/v1/messages", "type", "error"},
		{"/anthropic/v1/messages", "error.type", "rate_limit_error"},
		{"/gemini/v1beta/models/test:generateContent", "error.status", "RESOURCE_EXHAUSTED"},
		{"/v1beta/models/test:streamGenerateContent", "error.code", "429"},
	} {
		router.POST(tc.path, func(*gin.Context) { t.Error("rejected request reached the handler") })
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, tc.path, nil))
		require.Equal(t, http.StatusTooManyRequests, recorder.Code, tc.path)
		require.Equal(t, "1", recorder.Header().Get("Retry-After"))
		require.Equal(t, tc.value, gjson.GetBytes(recorder.Body.Bytes(), tc.field).String())
	}
}
