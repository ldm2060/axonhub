package biz

import (
	"testing"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/stretchr/testify/require"
)

func TestResolveChannelAutoDisableConfig(t *testing.T) {
	globalPolicy := &RetryPolicy{
		AutoDisableChannel: AutoDisableChannel{
			Enabled: true,
			Statuses: []objects.AutoDisableChannelStatus{
				{Status: 401, Times: 3},
			},
		},
	}

	tests := []struct {
		name            string
		channelConfig   *objects.ChannelAutoDisableConfig
		expectedEnabled bool
		expectedCount   int
	}{
		{
			name:            "Nil config inherits global",
			channelConfig:   nil,
			expectedEnabled: true,
			expectedCount:   1,
		},
		{
			name: "Disabled mode",
			channelConfig: &objects.ChannelAutoDisableConfig{
				Mode: objects.AutoDisableModeDisabled,
			},
			expectedEnabled: false,
			expectedCount:   0,
		},
		{
			name: "Custom mode enabled",
			channelConfig: &objects.ChannelAutoDisableConfig{
				Mode:    objects.AutoDisableModeCustom,
				Enabled: true,
				Statuses: []objects.AutoDisableChannelStatus{
					{Status: 402, Times: 5},
					{Status: 403, Times: 2},
				},
			},
			expectedEnabled: true,
			expectedCount:   2,
		},
		{
			name: "Custom mode disabled",
			channelConfig: &objects.ChannelAutoDisableConfig{
				Mode:    objects.AutoDisableModeCustom,
				Enabled: false,
				Statuses: []objects.AutoDisableChannelStatus{
					{Status: 402, Times: 5},
				},
			},
			expectedEnabled: false,
			expectedCount:   0,
		},
		{
			name: "Inherit global mode explicit",
			channelConfig: &objects.ChannelAutoDisableConfig{
				Mode: objects.AutoDisableModeInheritGlobal,
			},
			expectedEnabled: true,
			expectedCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &ent.Channel{AutoDisableConfig: tt.channelConfig}
			enabled, statuses := ResolveChannelAutoDisableConfig(channel, globalPolicy)
			require.Equal(t, tt.expectedEnabled, enabled)
			require.Len(t, statuses, tt.expectedCount)
		})
	}
}
