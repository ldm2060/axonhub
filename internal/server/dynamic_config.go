package server

import (
	"github.com/ldm2060/axonhub/internal/server/middleware"
)

// NewIPAccessControlRuntime creates the single mutable runtime state used by
// the IP access-control middleware. SIGHUP reload handling updates this
// object directly instead of rebuilding routes or introducing a global
// configuration event bus.
func NewIPAccessControlRuntime(cfg Config) (*middleware.IPAccessControlConfig, error) {
	return middleware.NewIPAccessControlConfig(
		cfg.IPAccessControl.Enabled,
		cfg.IPAccessControl.AllowedIPs,
		cfg.IPAccessControl.RedirectURL,
	)
}

// NewConcurrencyLimitRuntime creates the mutable cap enforced by the per-user
// concurrency-limit middleware. The limit itself lives in the system settings
// table (see admin → 系统 → 常规); this object only holds the live snapshot.
func NewConcurrencyLimitRuntime() *middleware.ConcurrencyLimitConfig {
	return middleware.NewConcurrencyLimitConfig(0)
}
