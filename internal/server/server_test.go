package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPServerDisablesWriteTimeoutForLongStreams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	srv := &Server{
		Config: Config{
			Host:                 "127.0.0.1",
			Port:                 8090,
			ReadTimeout:          5 * time.Second,
			RequestTimeout:       30 * time.Second,
			LLMRequestTimeout:    10 * time.Minute,
			LLMStreamIdleTimeout: 2 * time.Minute,
		},
		Engine: gin.New(),
	}

	httpServer := srv.newHTTPServer("127.0.0.1:8090")

	require.Equal(t, 5*time.Second, httpServer.ReadTimeout)
	require.Zero(t, httpServer.WriteTimeout)
	require.IsType(t, &http.Server{}, httpServer)
}
