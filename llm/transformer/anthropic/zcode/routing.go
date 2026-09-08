// Endpoint routing — mirrors the ZCode client's ProviderEndpointRoutingService.
//
// The desktop client periodically fetches GET zcode.z.ai/api/v1/agent/configs
// and rewrites request URLs per the returned data.proxyEndpoint.mapping table
// (from → to, exact normalized match). As of 2026-09 the coding-plan Anthropic
// endpoints map to zcode.z.ai /api/v1/ultra[...]; the table is server-side and
// may change, so resolution stays generic. Any fetch/parse failure keeps the
// previous snapshot (or none) — requests then go to their original URL.

package zcode

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
)

const (
	// AgentConfigsURL is the server-controlled routing + feature-gate config.
	//nolint:gosec // false alert.
	AgentConfigsURL = "https://zcode.z.ai/api/v1/agent/configs"

	routingRefreshInterval = 5 * time.Minute
	routingRequestTimeout  = 3 * time.Second
	routingMaxEntries      = 256
)

// EndpointRouting resolves provider URLs through the server's mapping table.
// Safe for concurrent use.
type EndpointRouting struct {
	httpClient *httpclient.HttpClient

	mu        sync.RWMutex
	mapping   map[string]*url.URL
	fetchedAt time.Time
	fetching  chan struct{}
}

// NewEndpointRouting creates a routing resolver.
func NewEndpointRouting(httpClient *httpclient.HttpClient) *EndpointRouting {
	return &EndpointRouting{
		httpClient: httpClient,
		mu:         sync.RWMutex{},
		mapping:    nil,
		fetchedAt:  time.Time{},
		fetching:   nil,
	}
}

type agentConfigsEnvelope struct {
	Code *int `json:"code"`
	Data struct {
		ProxyEndpoint struct {
			Mapping []struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"mapping"`
		} `json:"proxyEndpoint"`
	} `json:"data"`
}

// resolve returns the mapped target for the URL, or nil when unmapped.
func (r *EndpointRouting) resolve(target *url.URL) *url.URL {
	if r == nil || target == nil {
		return nil
	}

	if r.shouldRefresh() {
		go r.refresh(context.Background())
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	if mapped, ok := r.mapping[routingKey(target)]; ok {
		clone := *mapped
		clone.RawQuery = target.RawQuery
		return &clone
	}
	return nil
}

func (r *EndpointRouting) shouldRefresh() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return time.Since(r.fetchedAt) > routingRefreshInterval
}

func (r *EndpointRouting) refresh(ctx context.Context) {
	r.mu.Lock()
	if r.fetching != nil {
		r.mu.Unlock()
		<-r.fetching
		return
	}
	r.fetching = make(chan struct{})
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		close(r.fetching)
		r.fetching = nil
		r.mu.Unlock()
	}()

	rctx, cancel := context.WithTimeout(ctx, routingRequestTimeout)
	defer cancel()

	req := &httpclient.Request{ //nolint:exhaustruct_v5 // only the request plumbing matters here.
		Method: http.MethodGet,
		URL:    AgentConfigsURL,
		Headers: headersWithIdentity(map[string][]string{
			"Accept":       {"application/json"},
			"User-Agent":   {"ZCode/" + AppVersion},
			"Http-Referer": {"https://zcode.z.ai"},
		}),
	}

	resp, err := r.httpClient.Do(rctx, req)
	if err != nil {
		slog.WarnContext(ctx, "zcode endpoint routing fetch failed", "error", err)
		return
	}

	var envelope agentConfigsEnvelope
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		slog.WarnContext(ctx, "zcode endpoint routing parse failed", "error", err)
		return
	}
	if envelope.Code == nil || *envelope.Code != 0 || len(envelope.Data.ProxyEndpoint.Mapping) > routingMaxEntries {
		return
	}

	mapping := make(map[string]*url.URL, len(envelope.Data.ProxyEndpoint.Mapping))
	for _, entry := range envelope.Data.ProxyEndpoint.Mapping {
		from, err := url.Parse(entry.From)
		if err != nil || from.Scheme != "https" {
			continue
		}
		to, err := url.Parse(entry.To)
		if err != nil || to.Scheme != "https" {
			continue
		}
		mapping[routingKey(from)] = to
	}

	r.mu.Lock()
	r.mapping = mapping
	r.fetchedAt = time.Now()
	r.mu.Unlock()
}

// routingKey normalizes scheme://host/path for exact-match lookups (ports and
// trailing slashes folded the way the client's table is keyed).
func routingKey(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	if port := u.Port(); port != "" && port != "443" {
		host += ":" + port
	}
	path := u.Path
	if path != "/" {
		path = strings.TrimRight(path, "/")
		if path == "" {
			path = "/"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + host + path
}

func headersWithIdentity(base map[string][]string) http.Header {
	return http.Header(base)
}
