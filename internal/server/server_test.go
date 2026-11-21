// 职责: 校验服务端写超时预算按请求/LLM 超时推导，长流式连接靠零预算豁免。
// 关联文件:
//   internal/server/config.go
//   internal/server/server.go
//   internal/server/middleware/timeout.go
// 注意事项:
//   LLM 路由预算为 0 时不得产生写超时；非零预算直接来自 writeTimeout()。

package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWriteTimeoutBoundsLongStreams(t *testing.T) {
	config := Config{
		RequestTimeout:          30 * time.Second,
		LLMRequestTimeout:       10 * time.Minute,
		LLMStreamRequestTimeout: 30 * time.Minute,
	}

	require.Equal(t, 30*time.Minute, config.writeTimeout())
}

func TestWriteTimeoutZeroBudgetStaysUnlimited(t *testing.T) {
	// Zero streaming budget must not silently pick up another timeout:
	// the server clears the write deadline for such LLM routes instead.
	config := Config{
		RequestTimeout:          30 * time.Second,
		LLMRequestTimeout:       10 * time.Minute,
		LLMStreamRequestTimeout: 0,
	}

	require.Equal(t, 10*time.Minute, config.writeTimeout())

	allZero := Config{}
	require.Zero(t, allZero.writeTimeout())
}
