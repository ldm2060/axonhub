package biz

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateChannelBaseURL checks that a channel base URL is a well-formed
// http(s)/ws(s) URL with a host. It intentionally does not restrict which
// hosts are allowed: pointing channels at self-hosted upstreams on localhost,
// the LAN, or container-internal service names is a supported deployment.
func ValidateChannelBaseURL(baseURL string) error {
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

	if parsed.Hostname() == "" {
		return fmt.Errorf("invalid base_url: missing host")
	}

	return nil
}
