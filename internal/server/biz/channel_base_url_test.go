package biz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateChannelBaseURL_RejectsUnsupportedSchemes(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "gopher://example.com", "ftp://example.com"} {
		require.Error(t, ValidateChannelBaseURL(raw), "expected %q to be rejected", raw)
	}
}

func TestValidateChannelBaseURL_AllowsAnyHost(t *testing.T) {
	// Internal and container-internal upstreams are legitimate channel targets.
	for _, raw := range []string{
		"https://api.openai.com/v1",
		"https://api.anthropic.com",
		"wss://example.com/stream",
		"http://sub2api:8080/v1",
		"http://127.0.0.1:11434",
		"http://localhost:11434",
		"http://10.0.0.5:8080",
		"", // unset base_url falls back to the provider default
	} {
		require.NoError(t, ValidateChannelBaseURL(raw), "expected %q to be allowed", raw)
	}
}
