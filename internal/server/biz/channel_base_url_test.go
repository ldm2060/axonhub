package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
)

func TestValidateChannelBaseURL_RejectsInternalTargetsForNonAdmins(t *testing.T) {
	// A user that may only manage their own channels — the self-registration default.
	ctx := contexts.WithUser(context.Background(), &ent.User{
		ID:     42,
		Scopes: []string{"manage_own_channels", "read_channels"},
	})

	blocked := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1:8090",
		"http://localhost:11434",
		"http://10.0.0.5:8080",
		"http://192.168.1.10",
		"http://172.16.0.1",
		"http://[::1]:8090",
		"http://100.64.0.1",
		"http://0.0.0.0",
		"http://metadata.internal",
	}

	for _, raw := range blocked {
		require.Error(t, ValidateChannelBaseURL(ctx, raw), "expected %q to be rejected", raw)
	}
}

func TestValidateChannelBaseURL_RejectsUnsupportedSchemes(t *testing.T) {
	ctx := contexts.WithUser(context.Background(), &ent.User{
		ID:     42,
		Scopes: []string{"manage_own_channels"},
	})

	for _, raw := range []string{"file:///etc/passwd", "gopher://example.com", "ftp://example.com"} {
		require.Error(t, ValidateChannelBaseURL(ctx, raw), "expected %q to be rejected", raw)
	}
}

func TestValidateChannelBaseURL_AllowsPublicTargets(t *testing.T) {
	ctx := contexts.WithUser(context.Background(), &ent.User{
		ID:     42,
		Scopes: []string{"manage_own_channels"},
	})

	for _, raw := range []string{
		"https://api.openai.com/v1",
		"https://api.anthropic.com",
		"wss://example.com/stream",
		"", // unset base_url falls back to the provider default
	} {
		require.NoError(t, ValidateChannelBaseURL(ctx, raw), "expected %q to be allowed", raw)
	}
}

func TestValidateChannelBaseURL_AllowsPrivateTargetsForAdmins(t *testing.T) {
	// Operators legitimately front self-hosted upstreams (Ollama, vLLM) on localhost.
	adminCtx := contexts.WithUser(context.Background(), &ent.User{
		ID:     1,
		Scopes: []string{"write_channels"},
	})
	require.NoError(t, ValidateChannelBaseURL(adminCtx, "http://127.0.0.1:11434"))

	ownerCtx := contexts.WithUser(context.Background(), &ent.User{ID: 1, IsOwner: true})
	require.NoError(t, ValidateChannelBaseURL(ownerCtx, "http://10.0.0.5:8080"))

	// Background/system writes carry no user and must not be blocked.
	require.NoError(t, ValidateChannelBaseURL(context.Background(), "http://127.0.0.1:11434"))
}
