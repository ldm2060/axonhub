package biz

import (
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/objects"
)

// ResolveChannelAutoDisableConfig resolves the effective auto-disable configuration
// for a channel by merging channel-level and global settings
func ResolveChannelAutoDisableConfig(
	channel *ent.Channel,
	globalPolicy *RetryPolicy,
) (enabled bool, statuses []objects.AutoDisableChannelStatus) {
	// Channel has custom configuration
	if channel.AutoDisableConfig != nil {
		switch channel.AutoDisableConfig.Mode {
		case objects.AutoDisableModeDisabled:
			return false, nil
		case objects.AutoDisableModeCustom:
			if channel.AutoDisableConfig.Enabled {
				return true, channel.AutoDisableConfig.Statuses
			}
			return false, nil
		case objects.AutoDisableModeInheritGlobal:
			// Explicit inherit, fall through to global
		default:
			// Unknown mode, fall through to global
		}
	}

	// Inherit global configuration
	return globalPolicy.AutoDisableChannel.Enabled,
		globalPolicy.AutoDisableChannel.Statuses
}
