package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/pkg/xcache/live"
	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
	"github.com/ldm2060/axonhub/internal/server/biz/usage_monitor"
	"github.com/ldm2060/axonhub/internal/server/scheduler"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
	"github.com/ldm2060/axonhub/llm/transformer/antigravity"
)

func assembleHeadersFromAPIKey(apiKey string, headerFormat string) map[string]any {
	// Check if apiKey is OAuth JSON (contains access_token field)
	var oauth struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal([]byte(apiKey), &oauth); err == nil && oauth.AccessToken != "" {
		return map[string]any{"Authorization": "Bearer " + oauth.AccessToken}
	}

	switch headerFormat {
	case "bearer":
		return map[string]any{"Authorization": "Bearer " + apiKey}
	case "x-api-key":
		return map[string]any{"x-api-key": apiKey}
	case "url_key":
		// API key is appended to URL as ?key= parameter, no auth headers needed
		return map[string]any{}
	default:
		return map[string]any{"Authorization": "Bearer " + apiKey}
	}
}

// convertMapSliceToVariables converts []map[string]any (DB storage) to []Variable.
func convertMapSliceToVariables(maps []map[string]any) []usage_monitor.Variable {
	result := make([]usage_monitor.Variable, 0, len(maps))
	for _, m := range maps {
		v := usage_monitor.Variable{}
		if key, ok := m["key"].(string); ok {
			v.Key = key
		}
		if path, ok := m["path"].(string); ok {
			v.Path = path
		}
		if typ, ok := m["type"].(string); ok {
			v.Type = typ
		}
		if gi, ok := m["groupIndex"]; ok {
			if arr, ok := gi.([]any); ok {
				for _, item := range arr {
					if n, ok := item.(float64); ok {
						v.GroupIndex = append(v.GroupIndex, int(n))
					} else if n, ok := item.(int); ok {
						v.GroupIndex = append(v.GroupIndex, n)
					}
				}
			}
		}
		result = append(result, v)
	}
	return result
}

// convertMapSliceToDisplayFields converts []map[string]any (DB storage) to []DisplayField.
func convertMapSliceToDisplayFields(maps []map[string]any) []usage_monitor.DisplayField {
	result := make([]usage_monitor.DisplayField, 0, len(maps))
	for _, m := range maps {
		d := usage_monitor.DisplayField{}
		if key, ok := m["key"].(string); ok {
			d.Key = key
		}
		if label, ok := m["label"].(string); ok {
			d.Label = label
		}
		if ref, ok := m["valueRef"].(string); ok {
			d.ValueRef = ref
		}
		if format, ok := m["format"].(string); ok {
			d.Format = format
		}
		if unit, ok := m["unit"].(string); ok {
			d.Unit = unit
		}
		if totalRef, ok := m["totalRef"].(string); ok {
			d.TotalRef = totalRef
		}
		if order, ok := m["displayOrder"]; ok {
			if n, ok := order.(float64); ok {
				d.DisplayOrder = int(n)
			} else if n, ok := order.(int); ok {
				d.DisplayOrder = n
			}
		}
		if badge, ok := m["badge"].(string); ok {
			d.Badge = badge
		}
		if presets, ok := m["badgePresets"].(string); ok {
			d.BadgePresets = presets
		}
		if group, ok := m["group"].(string); ok {
			d.Group = group
		}
		if groupLabelRef, ok := m["groupLabelRef"].(string); ok {
			d.GroupLabelRef = groupLabelRef
		}
		result = append(result, d)
	}
	return result
}

// enrichDisplayFieldsFromTemplate fills missing group/badge fields from the provider template.
func enrichDisplayFieldsFromTemplate(ch *ent.UsageMonitorChannel, dfs []usage_monitor.DisplayField) {
	if ch.Source != usagemonitorchannel.SourceTemplate || ch.ProviderType == "" {
		return
	}
	tmpl := usage_monitor.GetChannelTemplate(string(ch.ProviderType))
	if tmpl == nil {
		return
	}
	tmplDFMap := make(map[string]usage_monitor.DisplayField, len(tmpl.DisplayFields))
	for _, df := range tmpl.DisplayFields {
		tmplDFMap[df.Key] = df
	}
	for i := range dfs {
		tdf, ok := tmplDFMap[dfs[i].Key]
		if !ok {
			continue
		}
		if dfs[i].ValueRef == dfs[i].Key && tdf.ValueRef != "" && tdf.ValueRef != tdf.Key {
			dfs[i].ValueRef = tdf.ValueRef
		}
		if dfs[i].Badge == "" && tdf.Badge != "" {
			dfs[i].Badge = tdf.Badge
		}
		if dfs[i].BadgePresets == "" && tdf.BadgePresets != "" {
			dfs[i].BadgePresets = tdf.BadgePresets
		}
		if dfs[i].Group == "" && tdf.Group != "" {
			dfs[i].Group = tdf.Group
		}
		if dfs[i].GroupLabelRef == "" && tdf.GroupLabelRef != "" {
			dfs[i].GroupLabelRef = tdf.GroupLabelRef
		}
	}
}

// convertMapSliceToFieldConfigs converts []map[string]any (legacy DB storage) to []FieldConfig.
func convertMapSliceToFieldConfigs(maps []map[string]any) []usage_monitor.FieldConfig {
	result := make([]usage_monitor.FieldConfig, 0, len(maps))
	for _, f := range maps {
		fc := usage_monitor.FieldConfig{}
		if v, ok := f["key"].(string); ok {
			fc.Key = v
		}
		if v, ok := f["label"].(string); ok {
			fc.Label = v
		}
		if v, ok := f["path"].(string); ok {
			fc.Path = v
		}
		if v, ok := f["type"].(string); ok {
			fc.Type = v
		}
		if v, ok := f["format"].(string); ok {
			fc.Format = v
		}
		if v, ok := f["totalPath"].(string); ok {
			fc.TotalPath = v
		}
		if v, ok := f["unit"].(string); ok {
			fc.Unit = v
		}
		if v, ok := f["displayOrder"]; ok {
			if n, ok := v.(float64); ok {
				fc.DisplayOrder = int(n)
			} else if n, ok := v.(int); ok {
				fc.DisplayOrder = n
			}
		}
		if v, ok := f["groupIndex"]; ok {
			if arr, ok := v.([]any); ok {
				for _, item := range arr {
					if n, ok := item.(float64); ok {
						fc.GroupIndex = append(fc.GroupIndex, int(n))
					} else if n, ok := item.(int); ok {
						fc.GroupIndex = append(fc.GroupIndex, n)
					}
				}
			}
		}
		if v, ok := f["expression"].(string); ok {
			fc.Expression = v
		}
		result = append(result, fc)
	}
	return result
}

// convertVariablesToMapSlice converts []Variable to []map[string]any for DB storage.
func convertVariablesToMapSlice(vars []usage_monitor.Variable) []map[string]any {
	result := make([]map[string]any, 0, len(vars))
	for _, v := range vars {
		m := map[string]any{
			"key":  v.Key,
			"path": v.Path,
			"type": v.Type,
		}
		if len(v.GroupIndex) > 0 {
			m["groupIndex"] = v.GroupIndex
		}
		result = append(result, m)
	}
	return result
}

// convertDisplayFieldsToMapSlice converts []DisplayField to []map[string]any for DB storage.
func convertDisplayFieldsToMapSlice(dfs []usage_monitor.DisplayField) []map[string]any {
	result := make([]map[string]any, 0, len(dfs))
	for _, d := range dfs {
		m := map[string]any{
			"key":          d.Key,
			"label":        d.Label,
			"valueRef":     d.ValueRef,
			"format":       d.Format,
			"displayOrder": d.DisplayOrder,
		}
		if d.Unit != "" {
			m["unit"] = d.Unit
		}
		if d.TotalRef != "" {
			m["totalRef"] = d.TotalRef
		}
		if d.Badge != "" {
			m["badge"] = d.Badge
		}
		if d.BadgePresets != "" {
			m["badgePresets"] = d.BadgePresets
		}
		if d.Group != "" {
			m["group"] = d.Group
		}
		if d.GroupLabelRef != "" {
			m["groupLabelRef"] = d.GroupLabelRef
		}
		result = append(result, m)
	}
	return result
}

// convertFieldConfigsToMapSlice converts []FieldConfig (legacy) to []map[string]any for DB storage.
func convertFieldConfigsToMapSlice(fcs []usage_monitor.FieldConfig) []map[string]any {
	result := make([]map[string]any, 0, len(fcs))
	for _, fc := range fcs {
		m := map[string]any{
			"key":          fc.Key,
			"label":        fc.Label,
			"path":         fc.Path,
			"type":         fc.Type,
			"format":       fc.Format,
			"displayOrder": fc.DisplayOrder,
		}
		if fc.TotalPath != "" {
			m["totalPath"] = fc.TotalPath
		}
		if fc.Unit != "" {
			m["unit"] = fc.Unit
		}
		if len(fc.GroupIndex) > 0 {
			m["groupIndex"] = fc.GroupIndex
		}
		if fc.Expression != "" {
			m["expression"] = fc.Expression
		}
		result = append(result, m)
	}
	return result
}

// convertNewToLegacyFields converts new Variables + DisplayFields back to legacy []map[string]any Fields format.
// This enables dual-write so that older code reading the fields column still works during migration.
func convertNewToLegacyFields(variablesMapSlice []map[string]any, displayFieldsMapSlice []map[string]any) []map[string]any {
	vars := convertMapSliceToVariables(variablesMapSlice)
	dfs := convertMapSliceToDisplayFields(displayFieldsMapSlice)

	// Build a map from variable key to variable for lookup
	varMap := make(map[string]usage_monitor.Variable, len(vars))
	for _, v := range vars {
		varMap[v.Key] = v
	}

	// Convert DisplayFields back to FieldConfig format
	result := make([]map[string]any, 0, len(dfs))
	for _, df := range dfs {
		fc := map[string]any{
			"key":          df.Key,
			"label":        df.Label,
			"format":       df.Format,
			"displayOrder": df.DisplayOrder,
		}
		if df.Unit != "" {
			fc["unit"] = df.Unit
		}
		if df.TotalRef != "" {
			fc["totalPath"] = df.TotalRef
		}
		if df.Group != "" {
			fc["group"] = df.Group
		}
		if df.GroupLabelRef != "" {
			fc["groupLabelRef"] = df.GroupLabelRef
		}
		if df.ValueRef != "" && df.ValueRef != df.Key {
			fc["expression"] = df.ValueRef
		}

		// Find matching variable for path/type
		if v, ok := varMap[df.Key]; ok {
			fc["path"] = v.Path
			fc["type"] = v.Type
			if len(v.GroupIndex) > 0 {
				fc["groupIndex"] = v.GroupIndex
			}
		}
		result = append(result, fc)
	}
	return result
}

type UsageMonitorServiceParams struct {
	fx.In

	Ent                         *ent.Client
	HttpClient                  *httpclient.HttpClient
	Scheduler                   *scheduler.Scheduler
	DefaultDisableThreshold     float64 `name:"quota_channel_binding.default_disable_threshold"`
	DefaultEnableThreshold      float64 `name:"quota_channel_binding.default_enable_threshold"`
	DefaultMultiMonitorStrategy string  `name:"quota_channel_binding.default_multi_monitor_strategy"`
}

type QuotaCacheCallback func(channelID int, quotaStatus string, ready bool, limits []map[string]any)

// ChannelsReloadCallback is invoked after a channel's quota_binding_ready state
// changes in the DB, so the owner (ChannelService) can synchronously refresh
// the enabled-channels cache. Without this, ProviderQuotaSelector keeps reading
// a stale QuotaBindingReady value until the cache's 1-minute refresh tick.
type ChannelsReloadCallback func(ctx context.Context, channelID int)

type UsageMonitorService struct {
	*AbstractService

	cache                       *live.IndexedCache[int, *ent.UsageMonitorChannel]
	genericChecker              *usage_monitor.GenericQuotaChecker
	clineChecker                *provider_quota.ClineQuotaChecker
	opencodeGoChecker           *provider_quota.OpenCodeGoQuotaChecker
	minimaxChecker              *provider_quota.MinimaxQuotaChecker
	httpClient                  *httpclient.HttpClient
	defaultDisableThreshold     float64
	defaultEnableThreshold      float64
	defaultMultiMonitorStrategy string
	mu                          sync.Mutex
	quotaCacheCallback          QuotaCacheCallback
	channelsReloadCallback      ChannelsReloadCallback
}

func NewUsageMonitorService(params UsageMonitorServiceParams) *UsageMonitorService {
	svc := &UsageMonitorService{
		AbstractService:             &AbstractService{db: params.Ent},
		genericChecker:              usage_monitor.NewGenericQuotaChecker(params.HttpClient),
		clineChecker:                provider_quota.NewClineQuotaChecker(params.HttpClient),
		opencodeGoChecker:           provider_quota.NewOpenCodeGoQuotaChecker(params.HttpClient),
		minimaxChecker:              provider_quota.NewMinimaxQuotaChecker(params.HttpClient),
		httpClient:                  params.HttpClient,
		defaultDisableThreshold:     params.DefaultDisableThreshold,
		defaultEnableThreshold:      params.DefaultEnableThreshold,
		defaultMultiMonitorStrategy: params.DefaultMultiMonitorStrategy,
	}

	svc.cache = live.NewIndexedCache(live.IndexedOptions[int, *ent.UsageMonitorChannel]{
		Name:            "usage_monitor_channels",
		TTL:             10 * time.Minute,
		RefreshInterval: 60 * time.Second,
		KeyFunc:         func(v *ent.UsageMonitorChannel) int { return v.ID },
		LoadOneFunc:     svc.loadOne,
		LoadSinceFunc:   svc.loadSince,
		DeletedFunc: func(v *ent.UsageMonitorChannel) bool {
			return v.DeletedAt != 0
		},
	})

	return svc
}

func (svc *UsageMonitorService) Start(ctx context.Context) error {
	if err := svc.cache.Load(ctx); err != nil {
		return err
	}

	// One-time cleanup: remove orphan quota monitor bindings left behind by
	// older versions that did not clean up bindings on channel/monitor delete.
	// Logged, never fatal — a cleanup issue must not block startup.
	if removed, err := svc.CleanupOrphanedBindings(ctx); err != nil {
		log.Warn(ctx, "failed to clean orphaned quota monitor bindings", log.Cause(err))
	} else if removed > 0 {
		log.Info(ctx, "cleaned orphaned quota monitor bindings", log.Int("removed", removed))
	}

	return nil
}

func (svc *UsageMonitorService) Stop() {
	svc.cache.Stop()
}

func (svc *UsageMonitorService) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "usage-monitor-poll",
		Description: "Poll all usage monitor channels periodically",
		CronExpr:    "*/1 * * * *",
		Timezone:    "UTC",
	}, svc.runPollAllScheduled)
}

// SetQuotaCacheCallback registers a callback invoked after a monitor's quota
// status changes, so the ProviderQuotaSelector cache can be updated immediately.
// Must be called during initialization only (before Start), not concurrently
// with pollChannel reads. The callback is read without synchronization in the
// poll hot-path; concurrent writes during polling would cause a data race.
func (svc *UsageMonitorService) SetQuotaCacheCallback(cb QuotaCacheCallback) {
	svc.quotaCacheCallback = cb
}

// SetChannelsReloadCallback registers a callback invoked after a channel's
// quota_binding_ready state is updated, so the ChannelService can refresh its
// in-memory enabled-channels cache immediately (mirrors the pattern used by
// markChannelUnavailable in channel_auto_disable.go).
// Must be called during initialization only (before Start), not concurrently
// with pollChannel reads. The callback is read without synchronization in the
// poll hot-path; concurrent writes during polling would cause a data race.
func (svc *UsageMonitorService) SetChannelsReloadCallback(cb ChannelsReloadCallback) {
	svc.channelsReloadCallback = cb
}

func (svc *UsageMonitorService) loadOne(ctx context.Context, id int) (*ent.UsageMonitorChannel, error) {
	client := svc.entFromContext(ctx)

	item, err := client.UsageMonitorChannel.Query().
		Where(
			usagemonitorchannel.IDEQ(id),
			usagemonitorchannel.DeletedAtEQ(0),
		).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, live.ErrKeyNotFound
		}
		return nil, err
	}

	return item, nil
}

func (svc *UsageMonitorService) loadSince(ctx context.Context, since time.Time) ([]*ent.UsageMonitorChannel, time.Time, error) {
	client := svc.entFromContext(ctx)

	q := client.UsageMonitorChannel.Query().
		Where(usagemonitorchannel.DeletedAtEQ(0))

	if !since.IsZero() {
		q = q.Where(usagemonitorchannel.UpdatedAtGT(since))
	}

	items, err := q.All(ctx)
	if err != nil {
		return nil, since, err
	}

	maxUpdated := since
	for _, item := range items {
		if item.UpdatedAt.After(maxUpdated) {
			maxUpdated = item.UpdatedAt
		}
	}

	return items, maxUpdated, nil
}

// ListChannels returns all active (non-soft-deleted) monitor channels from the database.
func (svc *UsageMonitorService) ListChannels(ctx context.Context) ([]*ent.UsageMonitorChannel, error) {
	client := svc.entFromContext(ctx)
	channels, err := client.UsageMonitorChannel.Query().
		Where(usagemonitorchannel.DeletedAtEQ(0)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	for _, ch := range channels {
		svc.cache.Set(ch.ID, ch)
	}

	return channels, nil
}

// ListChannelsFromCache returns all usage monitor channels from the in-memory cache.
// Used by ProviderQuotaService to populate its routing cache on startup.
func (svc *UsageMonitorService) ListChannelsFromCache() []*ent.UsageMonitorChannel {
	all := svc.cache.GetAll()
	result := make([]*ent.UsageMonitorChannel, 0, len(all))
	for _, ch := range all {
		result = append(result, ch)
	}
	return result
}

// GetChannel returns a single monitor channel by ID from cache.
func (svc *UsageMonitorService) GetChannel(ctx context.Context, id int) (*ent.UsageMonitorChannel, error) {
	return svc.cache.Get(ctx, id)
}

// CreateChannel creates a new usage monitor channel.
func (svc *UsageMonitorService) CreateChannel(ctx context.Context, input usage_monitor.CreateUsageMonitorChannelInput) (*ent.UsageMonitorChannel, error) {
	client := svc.entFromContext(ctx)

	// Parse apiHeaders from string to map[string]any
	var apiHeaders map[string]any
	if input.ApiHeaders != "" {
		if err := json.Unmarshal([]byte(input.ApiHeaders), &apiHeaders); err != nil {
			return nil, fmt.Errorf("invalid apiHeaders JSON: %w", err)
		}
	}

	// Source-aware validation and auto-fill
	var variablesMapSlice []map[string]any
	var displayFieldsMapSlice []map[string]any

	switch input.Source {
	case "template":
		if input.ProviderType == nil || *input.ProviderType == "" {
			return nil, fmt.Errorf("providerType is required when source=template")
		}
		tmpl := usage_monitor.GetChannelTemplate(*input.ProviderType)
		if tmpl == nil {
			return nil, fmt.Errorf("unknown provider template: %s", *input.ProviderType)
		}
		if *input.ProviderType == "cline" && (input.ChannelID == nil || *input.ChannelID == "") {
			return nil, fmt.Errorf("channelId is required for the cline template")
		}
		// Dedicated checkers read credentials from the bound channel at poll time.
		isBoundChannelProvider := *input.ProviderType == "opencode_go" || *input.ProviderType == "cline" || *input.ProviderType == "minimax"
		if !isBoundChannelProvider && (input.ApiKey == nil || *input.ApiKey == "") {
			return nil, fmt.Errorf("apiKey is required when source=template")
		}
		// Template provides API config
		input.ApiURL = tmpl.ApiURL
		input.ApiMethod = tmpl.ApiMethod
		if tmpl.ApiBody != "" {
			input.ApiBody = &tmpl.ApiBody
		}
		// Template provides variables/displayFields; user may override displayFields
		if len(input.Variables) == 0 {
			input.Variables = tmpl.Variables
		}
		if len(input.DisplayFields) == 0 {
			input.DisplayFields = tmpl.DisplayFields
		}
		// Assemble headers from apiKey + template headerFormat
		// Special case: Antigravity stores refreshToken|projectId, not a usable Bearer token.
		// The pollChannel method will refresh the token before each poll, so store empty headers.
		// Special case: OpenCode Go and Cline build credentials from their bound
		// channels at poll time, so store empty headers here.
		if *input.ProviderType == "antigravity" || isBoundChannelProvider {
			apiHeaders = map[string]any{}
		} else {
			apiHeaders = assembleHeadersFromAPIKey(*input.ApiKey, tmpl.HeaderFormat)
		}
		// For url_key format, append API key as query parameter to the URL
		if tmpl.HeaderFormat == "url_key" && input.ApiKey != nil {
			input.ApiURL = tmpl.ApiURL + "?key=" + *input.ApiKey
		}

	case "custom":
		if input.ApiURL == "" {
			return nil, fmt.Errorf("apiUrl is required when source=custom")
		}
		if len(input.Variables) == 0 && len(input.Fields) == 0 {
			return nil, fmt.Errorf("variables or fields are required when source=custom")
		}
		if len(input.DisplayFields) == 0 && len(input.Fields) == 0 {
			return nil, fmt.Errorf("displayFields or fields are required when source=custom")
		}

	case "builtin":
		if input.ChannelID == nil || *input.ChannelID == "" {
			return nil, fmt.Errorf("channelId is required when source=builtin")
		}
	}

	// Convert Variables and DisplayFields to []map[string]any for DB storage
	variablesMapSlice = convertVariablesToMapSlice(input.Variables)
	displayFieldsMapSlice = convertDisplayFieldsToMapSlice(input.DisplayFields)

	// Dual-write: also populate legacy fields column from new columns
	var fieldsMapSlice []map[string]any
	if len(variablesMapSlice) > 0 && len(displayFieldsMapSlice) > 0 {
		fieldsMapSlice = convertNewToLegacyFields(variablesMapSlice, displayFieldsMapSlice)
	} else {
		// Fall back to converting from legacy FieldConfig input
		fieldsMapSlice = convertFieldConfigsToMapSlice(input.Fields)
	}

	create := client.UsageMonitorChannel.Create().
		SetName(input.Name).
		SetSource(usagemonitorchannel.Source(input.Source)).
		SetAPIURL(input.ApiURL).
		SetAPIMethod(usagemonitorchannel.APIMethod(input.ApiMethod)).
		SetAPIHeaders(apiHeaders).
		SetPollInterval(input.PollInterval).
		SetFields(fieldsMapSlice).
		SetVariables(variablesMapSlice).
		SetDisplayFields(displayFieldsMapSlice).
		SetStatus(usagemonitorchannel.StatusActive)

	if input.ChannelID != nil && *input.ChannelID != "" {
		channelID, err := strconv.Atoi(*input.ChannelID)
		if err != nil {
			return nil, fmt.Errorf("invalid channelId: %w", err)
		}
		create.SetChannelID(channelID)
	}

	if input.ApiBody != nil {
		create.SetAPIBody(*input.ApiBody)
	}

	if apiHeaders == nil {
		create.SetAPIHeaders(map[string]any{})
	}

	if input.ProviderType != nil {
		create.SetProviderType(usagemonitorchannel.ProviderType(*input.ProviderType))
	}
	if input.ApiKey != nil {
		create.SetAPIKey(*input.ApiKey)
	}

	// Set owner from context
	if currentUser, ok := contexts.GetUser(ctx); ok && currentUser != nil {
		create.SetOwnerID(currentUser.ID)
	}

	ch, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create usage monitor channel: %w", err)
	}

	// Update cache
	svc.cache.Set(ch.ID, ch)

	return ch, nil
}

// UpdateChannel updates an existing usage monitor channel.
func (svc *UsageMonitorService) UpdateChannel(ctx context.Context, id int, input usage_monitor.UpdateUsageMonitorChannelInput) (*ent.UsageMonitorChannel, error) {
	client := svc.entFromContext(ctx)

	// Load existing channel for source-aware decisions
	existing, err := svc.GetChannel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage monitor channel: %w", err)
	}

	update := client.UsageMonitorChannel.UpdateOneID(id)

	if input.Name != nil {
		update.SetName(*input.Name)
	}

	if input.ApiURL != nil {
		update.SetAPIURL(*input.ApiURL)
	}

	if input.ApiMethod != nil {
		update.SetAPIMethod(usagemonitorchannel.APIMethod(*input.ApiMethod))
	}

	if input.ApiHeaders != nil {
		var apiHeaders map[string]any
		if *input.ApiHeaders != "" {
			if err := json.Unmarshal([]byte(*input.ApiHeaders), &apiHeaders); err != nil {
				return nil, fmt.Errorf("invalid apiHeaders JSON: %w", err)
			}
		}
		if apiHeaders == nil {
			apiHeaders = map[string]any{}
		}
		update.SetAPIHeaders(apiHeaders)
	}

	if input.ApiKey != nil {
		update.SetAPIKey(*input.ApiKey)
		// Re-assemble headers if this is a template channel
		if existing.Source == usagemonitorchannel.SourceTemplate {
			tmpl := usage_monitor.GetChannelTemplate(string(existing.ProviderType))
			if tmpl != nil {
				// For url_key format, update URL with new API key
				if tmpl.HeaderFormat == "url_key" {
					update.SetAPIURL(tmpl.ApiURL + "?key=" + *input.ApiKey)
				}
				// Special case: Antigravity stores refreshToken|projectId, not a usable Bearer token.
				// The pollChannel method will refresh the token before each poll, so store empty headers.
				if string(existing.ProviderType) == "antigravity" {
					update.SetAPIHeaders(map[string]any{})
				} else {
					apiHeaders := assembleHeadersFromAPIKey(*input.ApiKey, tmpl.HeaderFormat)
					update.SetAPIHeaders(apiHeaders)
				}
			}
		}
	}

	if input.ApiBody != nil {
		update.SetAPIBody(*input.ApiBody)
	}

	if input.PollInterval != nil {
		update.SetPollInterval(*input.PollInterval)
	}

	// Source-aware handling of Variables and DisplayFields
	// Template channels: variables are read-only (owned by the template)
	if existing.Source != usagemonitorchannel.SourceTemplate {
		if input.Variables != nil {
			varsMapSlice := convertVariablesToMapSlice(*input.Variables)
			update.SetVariables(varsMapSlice)
		}
	}

	// DisplayFields are always editable
	if input.DisplayFields != nil {
		dfsMapSlice := convertDisplayFieldsToMapSlice(*input.DisplayFields)
		update.SetDisplayFields(dfsMapSlice)
	}

	// Dual-write: if variables or displayFields changed, also update legacy fields
	if input.Variables != nil || input.DisplayFields != nil {
		// Determine effective variables and displayFields for the legacy conversion
		var effectiveVars []map[string]any
		var effectiveDFs []map[string]any

		if input.Variables != nil {
			effectiveVars = convertVariablesToMapSlice(*input.Variables)
		} else {
			effectiveVars = existing.Variables
		}

		if input.DisplayFields != nil {
			effectiveDFs = convertDisplayFieldsToMapSlice(*input.DisplayFields)
		} else {
			effectiveDFs = existing.DisplayFields
		}

		legacyFields := convertNewToLegacyFields(effectiveVars, effectiveDFs)
		update.SetFields(legacyFields)
	} else if input.Fields != nil {
		// Legacy path: update fields column directly if only fields provided
		update.SetFields(convertFieldConfigsToMapSlice(*input.Fields))
	}

	if input.Status != nil {
		update.SetStatus(usagemonitorchannel.Status(*input.Status))
	}

	ch, err := update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update usage monitor channel: %w", err)
	}

	// Refresh cache
	svc.cache.Set(ch.ID, ch)

	// Re-evaluate any channels that have bindings to this monitor
	// (status/config changes may affect their quota-ready state)
	svc.evaluateChannelsForMonitor(ctx, ch.ID)

	return ch, nil
}

// DeleteChannel soft-deletes a usage monitor channel.
func (svc *UsageMonitorService) DeleteChannel(ctx context.Context, id int) error {
	client := svc.entFromContext(ctx)

	now := time.Now().Unix()
	_, err := client.UsageMonitorChannel.UpdateOneID(id).
		SetDeletedAt(int(now)).
		SetStatus(usagemonitorchannel.StatusPaused).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete usage monitor channel: %w", err)
	}

	// Soft-delete quota monitor bindings that referenced this monitor so they
	// don't linger as orphans (an orphan binding surfaces in binding summaries
	// with an empty strategy and breaks the frontend zod schema).
	if err := svc.softDeleteBindingsForMonitor(ctx, id); err != nil {
		return err
	}

	// Remove from cache
	svc.cache.Invalidate(id)

	// Re-evaluate any channels that had bindings to this monitor
	svc.evaluateChannelsForMonitor(ctx, id)

	return nil
}

// TestConnection tests the connection to the specified API.
func (svc *UsageMonitorService) TestConnection(ctx context.Context, input usage_monitor.TestUsageMonitorChannelInput) *usage_monitor.TestResult {
	var apiHeaders map[string]any
	if input.ApiHeaders != "" {
		if err := json.Unmarshal([]byte(input.ApiHeaders), &apiHeaders); err != nil {
			return &usage_monitor.TestResult{
				Success: false,
				Error:   fmt.Sprintf("invalid apiHeaders JSON: %v", err),
			}
		}
	}

	// For Antigravity OAuth, refresh the access token before testing.
	if input.ProviderType != nil && *input.ProviderType == "antigravity" && input.ApiKey != nil && *input.ApiKey != "" {
		refreshedHeaders, err := svc.refreshAntigravityToken(ctx, *input.ApiKey)
		if err != nil {
			return &usage_monitor.TestResult{
				Success: false,
				Error:   fmt.Sprintf("Antigravity token refresh failed: %v", err),
			}
		}
		apiHeaders = refreshedHeaders
	}

	apiBody := ""
	if input.ApiBody != nil {
		apiBody = *input.ApiBody
	}

	// Use V2 pipeline when variables/displayFields are provided
	if len(input.Variables) > 0 || len(input.DisplayFields) > 0 {
		return svc.genericChecker.TestConnectionV2(ctx, input.ApiURL, input.ApiMethod, apiHeaders, apiBody, input.Variables, input.DisplayFields)
	}

	// Fallback to legacy fields-based pipeline
	fields := make([]usage_monitor.FieldConfig, len(input.Fields))
	copy(fields, input.Fields)

	return svc.genericChecker.TestConnection(ctx, input.ApiURL, input.ApiMethod, apiHeaders, apiBody, fields)
}

// RefreshChannel force-polls a single channel and updates its data.
func (svc *UsageMonitorService) RefreshChannel(ctx context.Context, id int) (*ent.UsageMonitorChannel, error) {
	ch, err := svc.GetChannel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage monitor channel: %w", err)
	}

	svc.pollChannel(ctx, ch)

	// Reload from DB to get the updated state
	updated, err := svc.cache.Reload(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to reload usage monitor channel after refresh: %w", err)
	}

	return updated, nil
}

// RunPollAll forces an immediate poll of all usage monitor channels.
// Used by ProviderQuotaService.ManualCheck() to trigger a manual refresh.
func (svc *UsageMonitorService) RunPollAll(ctx context.Context) {
	svc.runPollAll(ctx)
}

func (svc *UsageMonitorService) runPollAll(ctx context.Context) {
	all := svc.cache.GetAll()
	if len(all) == 0 {
		return
	}

	now := time.Now()
	pollable := make([]*ent.UsageMonitorChannel, 0, len(all))
	for _, ch := range all {
		if ch.Status == usagemonitorchannel.StatusPaused {
			continue
		}
		interval := time.Duration(ch.PollInterval) * time.Second
		if interval <= 0 {
			interval = 60 * time.Second
		}
		lastPoll := ch.LastPollAt
		if lastPoll == nil {
			lastPoll = &time.Time{}
		}
		if now.Sub(*lastPoll) >= interval {
			pollable = append(pollable, ch)
		}
	}

	log.Debug(ctx, "Polling usage monitor channels",
		log.Int("total", len(all)),
		log.Int("pollable", len(pollable)))

	for _, ch := range pollable {
		svc.pollChannel(ctx, ch)
	}
}

func (svc *UsageMonitorService) pollChannel(ctx context.Context, ch *ent.UsageMonitorChannel) {
	apiBody := ch.APIBody
	apiHeaders := ch.APIHeaders

	// For Antigravity OAuth channels, refresh the access token before polling.
	// The apiKey is stored as "refreshToken|projectId" and we need a fresh
	// access token for the Authorization header.
	if ch.Source == usagemonitorchannel.SourceTemplate &&
		ch.ProviderType == usagemonitorchannel.ProviderTypeAntigravity &&
		ch.APIKey != "" {
		refreshedHeaders, err := svc.refreshAntigravityToken(ctx, ch.APIKey)
		if err != nil {
			log.Error(ctx, "Failed to refresh Antigravity token for usage monitor channel",
				log.Int("channel_id", ch.ID),
				log.String("channel_name", ch.Name),
				log.Cause(err))

			// Update DB with error status
			client := svc.entFromContext(ctx)
			errMsg := fmt.Sprintf("Antigravity token refresh failed: %v", err)
			_, updateErr := client.UsageMonitorChannel.UpdateOneID(ch.ID).
				SetLastPollError(errMsg).
				SetStatus(usagemonitorchannel.StatusError).
				Save(ctx)
			if updateErr != nil {
				log.Error(ctx, "Failed to update error status for usage monitor channel",
					log.Int("channel_id", ch.ID),
					log.Cause(updateErr))
			}
			svc.cache.Invalidate(ch.ID)
			// Re-evaluate bound channels since monitor status changed to error
			svc.evaluateChannelsForMonitor(ctx, ch.ID)
			return
		}
		apiHeaders = refreshedHeaders
	}

	var pollData *usage_monitor.PollData
	var err error

	// Cline, OpenCode Go, and Minimax use dedicated checkers that read credentials from the
	// bound channel, bypassing the generic HTTP poller entirely.
	if ch.ProviderType == usagemonitorchannel.ProviderTypeCline {
		pollData, err = svc.pollCline(ctx, ch)
	} else if ch.ProviderType == usagemonitorchannel.ProviderTypeOpencodeGo {
		pollData, err = svc.pollOpenCodeGo(ctx, ch)
	} else if ch.ProviderType == usagemonitorchannel.ProviderTypeMinimax {
		pollData, err = svc.pollMinimax(ctx, ch)
	} else if len(ch.Variables) > 0 && len(ch.DisplayFields) > 0 {
		// Prefer new columns (Variables + DisplayFields) if populated
		vars := convertMapSliceToVariables(ch.Variables)
		dfs := convertMapSliceToDisplayFields(ch.DisplayFields)
		enrichDisplayFieldsFromTemplate(ch, dfs)
		pollData, err = svc.genericChecker.PollV2(ctx, ch.APIURL, string(ch.APIMethod), apiHeaders, apiBody, vars, dfs)
	} else {
		// Fall back to legacy Fields column
		fieldConfigs := convertMapSliceToFieldConfigs(ch.Fields)
		pollData, err = svc.genericChecker.Poll(ctx, ch.APIURL, string(ch.APIMethod), apiHeaders, apiBody, fieldConfigs)
	}
	if err != nil {
		log.Error(ctx, "Failed to poll usage monitor channel",
			log.Int("channel_id", ch.ID),
			log.String("channel_name", ch.Name),
			log.Cause(err))

		// Update DB with error status
		client := svc.entFromContext(ctx)
		errMsg := err.Error()
		_, updateErr := client.UsageMonitorChannel.UpdateOneID(ch.ID).
			SetLastPollError(errMsg).
			SetStatus(usagemonitorchannel.StatusError).
			Save(ctx)
		if updateErr != nil {
			log.Error(ctx, "Failed to update error status for usage monitor channel",
				log.Int("channel_id", ch.ID),
				log.Cause(updateErr))
		}

		// Invalidate cache to trigger reload
		svc.cache.Invalidate(ch.ID)
		// Re-evaluate bound channels since monitor status changed to error
		svc.evaluateChannelsForMonitor(ctx, ch.ID)
		return
	}

	// Convert PollData to map for storage
	pollDataMap := map[string]any{
		"raw":      pollData.Raw,
		"polledAt": pollData.PolledAt.Format(time.RFC3339),
	}
	parsedFields := make([]map[string]any, 0, len(pollData.Fields))
	for _, f := range pollData.Fields {
		parsedFields = append(parsedFields, map[string]any{
			"key":        f.Key,
			"label":      f.Label,
			"value":      f.Value,
			"total":      f.Total,
			"percent":    f.Percent,
			"unit":       f.Unit,
			"format":     f.Format,
			"error":      f.Error,
			"group":      f.Group,
			"groupLabel": f.GroupLabel,
		})
	}
	pollDataMap["fields"] = parsedFields

	now := pollData.PolledAt

	// Derive quota status from parsed fields (works for all provider types including generic/custom)
	var quotaStatus string
	var quotaReady *bool
	var quotaLimits []map[string]any
	var nextResetAt *time.Time

	derived := usage_monitor.DeriveQuotaStatus(string(ch.ProviderType), pollData.Fields)
	quotaStatus = derived.Status
	quotaReady = &derived.Ready
	nextResetAt = derived.NextResetAt
	for _, l := range derived.Limits {
		m := map[string]any{
			"type":       string(l.Type),
			"status":     l.Status,
			"usageRatio": l.UsageRatio,
			"ready":      l.Ready,
		}
		if l.NextResetAt != nil {
			m["nextResetAt"] = l.NextResetAt.Format(time.RFC3339)
		}
		quotaLimits = append(quotaLimits, m)
	}

	// Evaluate auto-disable conditions if enabled
	if ch.AutoDisableEnabled {
		maxUsageRatio := calculateMaxUsageRatio(derived.Limits)
		disableThreshold := getDisableThreshold(ch, svc.defaultDisableThreshold)
		enableThreshold := getEnableThreshold(ch, svc.defaultEnableThreshold)

		// Get current quota_ready state, defaulting to true if nil
		currentQuotaReady := true
		if ch.QuotaReady != nil {
			currentQuotaReady = *ch.QuotaReady
		}

		newQuotaReady := evaluateQuotaReady(currentQuotaReady, maxUsageRatio, disableThreshold, enableThreshold)

		if newQuotaReady != currentQuotaReady {
			log.Info(ctx, "Auto-disable evaluation changed monitor quota_ready state",
				log.Int("monitor_id", ch.ID),
				log.String("monitor_name", ch.Name),
				log.Bool("old_quota_ready", currentQuotaReady),
				log.Bool("new_quota_ready", newQuotaReady),
				log.Float64("max_usage_ratio", maxUsageRatio),
				log.Float64("disable_threshold", disableThreshold),
				log.Float64("enable_threshold", enableThreshold))
			quotaReady = &newQuotaReady
		}
	}

	// Update DB with success
	client := svc.entFromContext(ctx)
	update := client.UsageMonitorChannel.UpdateOneID(ch.ID).
		SetLastPollData(pollDataMap).
		SetLastPollAt(now).
		SetStatus(usagemonitorchannel.StatusActive).
		ClearLastPollError()

	if quotaStatus != "" {
		update.SetQuotaStatus(usagemonitorchannel.QuotaStatus(quotaStatus))
	}
	if quotaReady != nil {
		update.SetQuotaReady(*quotaReady)
	}
	if len(quotaLimits) > 0 {
		update.SetQuotaLimits(quotaLimits)
	} else {
		update.ClearQuotaLimits()
	}
	if nextResetAt != nil {
		update.SetNextResetAt(*nextResetAt)
	} else {
		update.ClearNextResetAt()
	}

	updated, updateErr := update.Save(ctx)
	if updateErr != nil {
		log.Error(ctx, "Failed to update poll data for usage monitor channel",
			log.Int("channel_id", ch.ID),
			log.Cause(updateErr))
	} else {
		// Update cache with the new state
		svc.cache.Set(updated.ID, updated)

		// Notify quota cache for orchestrator routing
		if svc.quotaCacheCallback != nil && updated.ChannelID != nil && quotaStatus != "" {
			svc.quotaCacheCallback(*updated.ChannelID, quotaStatus, quotaReady != nil && *quotaReady, quotaLimits)
		}

		// Legacy direct-channel evaluation for builtin monitors with ChannelID.
		// Skip if active ChannelUsageMonitorBinding rows exist for this monitor,
		// because evaluateChannelsForMonitor (below) will already re-evaluate
		// every bound channel. Without this guard, a builtin monitor that has
		// been migrated to the binding model would evaluate the same channel
		// twice per poll.
		if updated.Source == usagemonitorchannel.SourceBuiltin && updated.ChannelID != nil {
			if !svc.hasActiveBindingsForMonitor(ctx, updated.ID) {
				if err := svc.evaluateAndUpdateChannelQuotaReady(ctx, *updated.ChannelID); err != nil {
					log.Error(ctx, "Failed to evaluate channel quota_ready status (legacy fallback)",
						log.Int("channel_id", *updated.ChannelID),
						log.Int("monitor_id", updated.ID),
						log.Cause(err))
				}
			}
		}

		// Re-evaluate all channels bound to this monitor regardless of source.
		// Custom/template monitors are bound exclusively through
		// ChannelUsageMonitorBinding and must also trigger re-evaluation
		// after a successful poll.
		svc.evaluateChannelsForMonitor(ctx, updated.ID)
	}

	log.Debug(ctx, "Polled usage monitor channel",
		log.Int("channel_id", ch.ID),
		log.String("channel_name", ch.Name),
		log.Int("fields_parsed", len(pollData.Fields)))
}

// refreshAntigravityToken takes an apiKey in "refreshToken|projectId" format,
// uses the refresh token to obtain a fresh access token, and returns headers
// suitable for making the quota API call.
func (svc *UsageMonitorService) refreshAntigravityToken(ctx context.Context, apiKey string) (map[string]any, error) {
	parts := strings.SplitN(apiKey, "|", 2)
	if len(parts) != 2 || parts[0] == "" {
		return nil, fmt.Errorf("invalid Antigravity credentials format, expected refreshToken|projectId")
	}
	refreshToken := parts[0]

	tokenProvider := antigravity.NewTokenProvider(oauth.TokenProviderParams{
		HTTPClient: svc.httpClient,
		Credentials: &oauth.OAuthCredentials{
			RefreshToken: refreshToken,
			ClientID:     antigravity.ClientID,
		},
	})

	creds, err := tokenProvider.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Antigravity token: %w", err)
	}

	return map[string]any{"Authorization": "Bearer " + creds.AccessToken}, nil
}

// pollCline polls a Cline channel via its dedicated API checker. Credentials,
// model scope, and proxy configuration come from the bound channel rather than
// the monitor's API key, preserving the channel as the single source of truth.
func (svc *UsageMonitorService) pollCline(ctx context.Context, mon *ent.UsageMonitorChannel) (*usage_monitor.PollData, error) {
	if mon.ChannelID == nil {
		return nil, fmt.Errorf("cline monitor %d has no bound channel_id", mon.ID)
	}

	client := svc.entFromContext(ctx)
	ch, err := client.Channel.Get(ctx, *mon.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("load bound channel %d for cline monitor: %w", *mon.ChannelID, err)
	}

	quota, err := svc.clineChecker.CheckQuota(ctx, ch)
	if err != nil {
		return nil, err
	}

	return &usage_monitor.PollData{
		Raw:      usage_monitor.ClineQuotaRawJSON(quota),
		Fields:   usage_monitor.ClineQuotaToParsedFields(quota),
		PolledAt: time.Now(),
	}, nil
}

func (svc *UsageMonitorService) pollMinimax(ctx context.Context, mon *ent.UsageMonitorChannel) (*usage_monitor.PollData, error) {
	if mon.ChannelID == nil {
		return nil, fmt.Errorf("minimax monitor %d has no bound channel_id", mon.ID)
	}

	client := svc.entFromContext(ctx)
	ch, err := client.Channel.Get(ctx, *mon.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("load bound channel %d for minimax monitor: %w", *mon.ChannelID, err)
	}

	quota, err := svc.minimaxChecker.CheckQuota(ctx, ch)
	if err != nil {
		return nil, err
	}

	return &usage_monitor.PollData{
		Raw:      usage_monitor.MinimaxQuotaRawJSON(quota),
		Fields:   usage_monitor.MinimaxQuotaToParsedFields(quota),
		PolledAt: time.Now(),
	}, nil
}

// pollOpenCodeGo polls an OpenCode Go channel via the dedicated dashboard-scraper
// checker. Credentials (workspaceId + authCookie) are read from the bound channel's
// settings.providerQuota.opencodeGo, so the monitor must have a channel_id. The
// returned QuotaData is converted to ParsedFields for the generic storage + display
// path; DeriveQuotaStatus then derives routing status from those fields.
func (svc *UsageMonitorService) pollOpenCodeGo(ctx context.Context, mon *ent.UsageMonitorChannel) (*usage_monitor.PollData, error) {
	if mon.ChannelID == nil {
		return nil, fmt.Errorf("opencode_go monitor %d has no bound channel_id", mon.ID)
	}

	client := svc.entFromContext(ctx)
	ch, err := client.Channel.Get(ctx, *mon.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("load bound channel %d for opencode_go monitor: %w", *mon.ChannelID, err)
	}

	quota, err := svc.opencodeGoChecker.CheckQuota(ctx, ch)
	if err != nil {
		return nil, err
	}

	return &usage_monitor.PollData{
		Raw:      usage_monitor.OpenCodeGoQuotaRawJSON(quota),
		Fields:   usage_monitor.OpenCodeGoQuotaToParsedFields(quota),
		PolledAt: time.Now(),
	}, nil
}
