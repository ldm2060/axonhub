package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

// WithIPBlocklist blocks external API requests from configured IP addresses or CIDR ranges.
func WithIPBlocklist(systemService *biz.SystemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIPs := blocklistIPCandidates(c)
		if len(clientIPs) == 0 {
			c.Next()
			return
		}

		ctx := authz.WithSystemBypass(c.Request.Context(), "ip-blocklist-middleware")
		settings := systemService.SecuritySettingsOrDefault(ctx)
		if !isBlockedIP(clientIPs, settings.BlockedIPs) {
			c.Next()
			return
		}

		AbortWithError(c, http.StatusForbidden, errors.New("IP address is blocked"))
	}
}

func clientIPCandidates(c *gin.Context) []string {
	clientIP := strings.TrimSpace(c.ClientIP())
	if clientIP == "" {
		return nil
	}

	return []string{clientIP}
}

// blocklistIPCandidates extends the trusted client IP with the same header-derived
// candidate the request log records (llm/httpclient.getClientIP: the first
// X-Forwarded-For entry, else X-Real-IP). A ban entered from the request-log UI holds
// that logged IP, so the blocklist must evaluate it too — otherwise a proxy in front of
// AxonHub makes every logged IP unmatchable. Untrusted header values are safe to match
// here, unlike in the allowlist paths that share clientIPCandidates: the blocklist can
// only over-block the requester itself, never grant access.
func blocklistIPCandidates(c *gin.Context) []string {
	candidates := clientIPCandidates(c)

	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}

		if slices.Contains(candidates, value) {
			return
		}

		candidates = append(candidates, value)
	}

	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		before, _, _ := strings.Cut(xff, ",")
		add(before)
	} else {
		add(c.Request.Header.Get("X-Real-IP"))
	}

	return candidates
}

func isBlockedIP(clientIPs []string, blockedIPs []string) bool {
	for _, clientIP := range clientIPs {
		clientAddr, err := netip.ParseAddr(clientIP)
		if err != nil {
			log.Warn(context.Background(), "failed to parse client IP", log.String("client_ip", clientIP), log.Cause(err))
			continue
		}

		if isBlockedAddr(clientAddr, blockedIPs) {
			return true
		}
	}

	return false
}

func isBlockedAddr(clientAddr netip.Addr, blockedIPs []string) bool {
	for _, item := range blockedIPs {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if strings.Contains(item, "/") {
			prefix, err := netip.ParsePrefix(item)
			if err != nil {
				log.Warn(context.Background(), "failed to parse blocked IP prefix", log.String("blocked_ip", item), log.Cause(err))
				continue
			}

			// Only prefixes within the IPv4-mapped address space can be
			// represented as IPv4 prefixes without changing their range.
			if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
				prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
			}
			if prefix.Contains(clientAddr) || prefix.Contains(clientAddr.Unmap()) {
				return true
			}

			continue
		}

		blockedAddr, err := netip.ParseAddr(item)
		if err != nil {
			log.Warn(context.Background(), "failed to parse blocked IP", log.String("blocked_ip", item), log.Cause(err))
			continue
		}

		if blockedAddr.Unmap() == clientAddr.Unmap() {
			return true
		}
	}

	return false
}

func isAnyAllowedIP(clientIPs []string, allowedIPs []string) bool {
	for _, clientIP := range clientIPs {
		clientAddr, err := netip.ParseAddr(clientIP)
		if err != nil {
			log.Warn(context.Background(), "failed to parse client IP", log.String("client_ip", clientIP), log.Cause(err))
			continue
		}

		if isBlockedAddr(clientAddr, allowedIPs) {
			return true
		}
	}

	return false
}
