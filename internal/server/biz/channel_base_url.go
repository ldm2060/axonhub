package biz

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/scopes"
)

// ValidateChannelBaseURL checks a channel base URL before it is persisted.
//
// The gateway issues server-side requests to this URL (chat proxying, channel tests,
// model sync), so an unrestricted value lets the caller reach anything the server can
// reach — cloud metadata endpoints, loopback admin ports, and other private services —
// and read the response back through the stored request execution.
//
// Operators legitimately point channels at self-hosted upstreams on localhost or the
// LAN (Ollama, vLLM, an internal gateway), so private destinations stay allowed for
// callers holding write_channels. Users who may only manage their own channels
// (manage_own_channels, part of the self-registration defaults) are restricted to
// public destinations.
func ValidateChannelBaseURL(ctx context.Context, baseURL string) error {
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}

	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}

	switch parsed.Scheme {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf("invalid base_url: unsupported scheme %q, expected http, https, ws or wss", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("invalid base_url: missing host")
	}

	// Administrators may target internal upstreams; everyone else may not.
	if channelAdminCanUsePrivateBaseURL(ctx) {
		return nil
	}

	return rejectPrivateHost(ctx, host)
}

// channelAdminCanUsePrivateBaseURL reports whether the caller is trusted to point a
// channel at a private/internal address.
func channelAdminCanUsePrivateBaseURL(ctx context.Context) bool {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		// Background jobs, migrations and system-initiated writes carry no user.
		return true
	}

	if user.IsOwner {
		return true
	}

	return scopes.UserHasScope(ctx, scopes.ScopeWriteChannels)
}

// rejectPrivateHost fails for hosts that resolve to a non-public address.
func rejectPrivateHost(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("invalid base_url: host %q points to a private or reserved address", host)
		}

		return nil
	}

	lowered := strings.ToLower(strings.TrimSuffix(host, "."))
	if lowered == "localhost" || strings.HasSuffix(lowered, ".localhost") || strings.HasSuffix(lowered, ".internal") {
		return fmt.Errorf("invalid base_url: host %q points to a private or reserved address", host)
	}

	// Resolution is advisory: a name that currently resolves into a private range is
	// rejected up front. A DNS-rebinding attempt that only flips after this check is
	// out of scope here and must be handled at dial time.
	//
	// Transient resolver failures must not block the write — the request itself will
	// fail later if the host is genuinely unreachable.
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil //nolint:nilerr // unresolvable hosts are handled at request time, not here
	}

	if slices.ContainsFunc(addrs, func(addr net.IPAddr) bool { return isPrivateIP(addr.IP) }) {
		return fmt.Errorf("invalid base_url: host %q resolves to a private or reserved address", host)
	}

	return nil
}

// isPrivateIP reports whether ip is loopback, link-local (including the cloud metadata
// address), private, or otherwise not a globally routable destination.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}

	// Carrier-grade NAT (100.64.0.0/10) fronts internal infrastructure on several
	// cloud providers and is not covered by net.IP.IsPrivate.
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
		// 0.0.0.0/8 and 240.0.0.0/4 are reserved.
		if v4[0] == 0 || v4[0] >= 240 {
			return true
		}
	}

	return false
}
