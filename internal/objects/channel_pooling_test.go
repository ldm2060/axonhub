package objects

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsValidPoolingWindow(t *testing.T) {
	for _, valid := range []string{"5h", "daily", "weekly", "monthly"} {
		require.True(t, IsValidPoolingWindow(valid), valid)
	}
	for _, invalid := range []string{"", "7d", "30d", "hourly", "WEEKLY"} {
		require.False(t, IsValidPoolingWindow(invalid), invalid)
	}
}

func TestChannelPoolingConfig_OwnerReservedPct(t *testing.T) {
	cfg := &ChannelPoolingConfig{
		Members: []ChannelPoolingMember{{UserID: 2, SharePct: 25}, {UserID: 3, SharePct: 15}},
	}
	require.InDelta(t, 60.0, cfg.OwnerReservedPct(), 1e-9)

	// Over-assignment clamps at zero; the validator rejects it on save but the
	// derived figure must never go negative.
	over := &ChannelPoolingConfig{
		Members: []ChannelPoolingMember{{UserID: 2, SharePct: 60}, {UserID: 3, SharePct: 60}},
	}
	require.InDelta(t, 0.0, over.OwnerReservedPct(), 1e-9)

	require.InDelta(t, 100.0, (&ChannelPoolingConfig{}).OwnerReservedPct(), 1e-9)
}

func TestChannelPoolingConfig_MemberByUserID(t *testing.T) {
	cfg := &ChannelPoolingConfig{
		Members: []ChannelPoolingMember{{UserID: 2, SharePct: 25}, {UserID: 3, SharePct: 15}},
	}
	member := cfg.MemberByUserID(3)
	require.NotNil(t, member)
	require.InDelta(t, 15.0, member.SharePct, 1e-9)

	require.Nil(t, cfg.MemberByUserID(4))
	// Mutation through the returned pointer hits the stored member.
	member.SharePct = 20
	require.InDelta(t, 20.0, cfg.Members[1].SharePct, 1e-9)
}
