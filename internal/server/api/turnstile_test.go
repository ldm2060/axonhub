package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

type stubTurnstileVerifier struct {
	verifyErr error
	config    *biz.PublicTurnstileConfig
	configErr error
}

func (v stubTurnstileVerifier) Verify(context.Context, string, string) error {
	return v.verifyErr
}

func (v stubTurnstileVerifier) PublicConfig(context.Context) (*biz.PublicTurnstileConfig, error) {
	if v.configErr != nil {
		return nil, v.configErr
	}

	return v.config, nil
}

func TestAuthHandlersConfigSanitizesPublicConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &biz.PublicTurnstileConfig{
		Enabled: true,
		SiteKey: "public-site-key",
	}
	config.Actions.SignIn = biz.TurnstileActionSignIn
	config.Actions.SignUpCode = biz.TurnstileActionSignUpCode
	handler := &AuthHandlers{
		TurnstileVerifier: stubTurnstileVerifier{config: config},
	}

	router := gin.New()
	router.GET("/auth/config", handler.Config)
	router.GET("/admin/auth/config", handler.Config)

	for _, path := range []string{"/auth/config", "/admin/auth/config"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			require.JSONEq(t, `{
				"turnstile": {
					"enabled": true,
					"site_key": "public-site-key",
					"actions": {
						"signin": "signin",
						"signup_send_code": "signup_send_code"
					}
				}
			}`, recorder.Body.String())
			require.NotContains(t, recorder.Body.String(), "secret")
			require.NotContains(t, recorder.Body.String(), "secretConfigured")
		})
	}
}

func TestAuthHandlersConfigFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &AuthHandlers{
		TurnstileVerifier: stubTurnstileVerifier{configErr: biz.ErrTurnstileUnavailable},
	}

	router := gin.New()
	router.GET("/auth/config", handler.Config)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/config", nil))

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.Contains(t, recorder.Body.String(), "Authentication configuration is temporarily unavailable")
	require.NotContains(t, recorder.Body.String(), "turnstile")
}

func TestAuthHandlersSignInTurnstileStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		verifyErr  error
		wantStatus int
		wantText   string
	}{
		{
			name:       "invalid challenge",
			verifyErr:  fmtTurnstileError(biz.ErrTurnstileInvalid),
			wantStatus: http.StatusBadRequest,
			wantText:   "Turnstile verification failed. Please try again",
		},
		{
			name:       "provider unavailable",
			verifyErr:  fmtTurnstileError(biz.ErrTurnstileUnavailable),
			wantStatus: http.StatusServiceUnavailable,
			wantText:   "Turnstile verification is temporarily unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &AuthHandlers{
				AuthService: &biz.AuthService{
					TurnstileVerifier: stubTurnstileVerifier{verifyErr: tt.verifyErr},
				},
			}

			router := gin.New()
			router.POST("/auth/signin", handler.SignIn)

			recorder := httptest.NewRecorder()
			body := bytes.NewBufferString(`{"email":"user@example.com","password":"password","turnstile_token":"token"}`)
			request := httptest.NewRequest(http.MethodPost, "/auth/signin", body)
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)

			require.Equal(t, tt.wantStatus, recorder.Code)
			require.Contains(t, recorder.Body.String(), tt.wantText)
			require.NotContains(t, recorder.Body.String(), "provider detail")
		})
	}
}

func TestAuthHandlersSignInPreservesInvalidCredentialStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()
	systemService := biz.NewSystemService(biz.SystemServiceParams{Ent: client})
	handler := &AuthHandlers{
		AuthService: &biz.AuthService{
			SystemService:     systemService,
			TurnstileVerifier: stubTurnstileVerifier{},
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := ent.NewContext(c.Request.Context(), client)
		ctx = authz.WithTestBypass(ctx)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/auth/signin", handler.SignIn)

	recorder := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"email":"user@example.com","password":"password"}`)
	request := httptest.NewRequest(http.MethodPost, "/auth/signin", body)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Invalid email or password")
}

func TestSignUpHandlersTurnstileStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantText   string
	}{
		{
			name:       "invalid challenge",
			err:        fmtTurnstileError(biz.ErrTurnstileInvalid),
			wantStatus: http.StatusBadRequest,
			wantText:   "Turnstile verification failed. Please try again",
		},
		{
			name:       "provider unavailable",
			err:        fmtTurnstileError(biz.ErrTurnstileUnavailable),
			wantStatus: http.StatusServiceUnavailable,
			wantText:   "Turnstile verification is temporarily unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/signup-error", func(c *gin.Context) {
				writeSignUpError(c, tt.err)
			})

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/signup-error", nil))

			require.Equal(t, tt.wantStatus, recorder.Code)
			require.Contains(t, recorder.Body.String(), tt.wantText)
			require.NotContains(t, recorder.Body.String(), "provider detail")
		})
	}
}

func TestWriteSignUpErrorPreservesExistingContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/signup-error", func(c *gin.Context) {
		writeSignUpError(c, errors.New("registration rejected"))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/signup-error", nil))

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, map[string]any{"error": "registration rejected"}, body)
}

func fmtTurnstileError(target error) error {
	return errors.Join(target, errors.New("provider detail must stay private"))
}
