package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/pkg/xcache/live"
	"github.com/ldm2060/axonhub/internal/server/biz/usage_monitor"
	"github.com/ldm2060/axonhub/internal/server/scheduler"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

func assembleHeadersFromAPIKey(apiKey string, headerFormat string) map[string]any {
	switch headerFormat {
	case "bearer":
		return map[string]any{"Authorization": "Bearer " + apiKey}
	case "x-api-key":
		return map[string]any{"x-api-key": apiKey}
	default:
		return map[string]any{"Authorization": "Bearer " + apiKey}
	}
}

type UsageMonitorServiceParams struct {
	fx.In

	Ent        *ent.Client
	HttpClient *httpclient.HttpClient
	Scheduler  *scheduler.Scheduler
}

type UsageMonitorService struct {
	*AbstractService

	cache          *live.IndexedCache[int, *ent.UsageMonitorChannel]
	genericChecker *usage_monitor.GenericQuotaChecker
	mu             sync.Mutex
}

func NewUsageMonitorService(params UsageMonitorServiceParams) *UsageMonitorService {
	svc := &UsageMonitorService{
		AbstractService: &AbstractService{db: params.Ent},
		genericChecker:  usage_monitor.NewGenericQuotaChecker(params.HttpClient),
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
	return svc.cache.Load(ctx)
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

// ListChannels returns all active (non-soft-deleted) monitor channels from cache.
func (svc *UsageMonitorService) ListChannels(ctx context.Context) ([]*ent.UsageMonitorChannel, error) {
	all := svc.cache.GetAll()
	result := make([]*ent.UsageMonitorChannel, 0, len(all))
	for _, ch := range all {
		result = append(result, ch)
	}
	return result, nil
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

	// Convert fields from []FieldConfig to []map[string]any
	fields := make([]map[string]any, 0, len(input.Fields))
	for _, f := range input.Fields {
		fieldMap := map[string]any{
			"key":          f.Key,
			"label":        f.Label,
			"path":         f.Path,
			"type":         f.Type,
			"format":       f.Format,
			"displayOrder": f.DisplayOrder,
		}
		if f.TotalPath != "" {
			fieldMap["totalPath"] = f.TotalPath
		}
		if f.Unit != "" {
			fieldMap["unit"] = f.Unit
		}
		if len(f.GroupIndex) > 0 {
			fieldMap["groupIndex"] = f.GroupIndex
		}
		fields = append(fields, fieldMap)
	}

	// Handle source=template: auto-fill from template registry
	if input.Source == "template" {
		if input.ProviderType == nil || *input.ProviderType == "" {
			return nil, fmt.Errorf("providerType is required when source=template")
		}
		if input.ApiKey == nil || *input.ApiKey == "" {
			return nil, fmt.Errorf("apiKey is required when source=template")
		}
		tmpl := usage_monitor.GetQuotaMonitorTemplate(*input.ProviderType)
		if tmpl == nil {
			return nil, fmt.Errorf("unknown provider template: %s", *input.ProviderType)
		}
		input.ApiURL = tmpl.ApiURL
		input.ApiMethod = tmpl.ApiMethod
		if tmpl.ApiBody != "" {
			input.ApiBody = &tmpl.ApiBody
		}
		input.Fields = tmpl.Fields

		// Assemble headers from apiKey
		apiHeaders = assembleHeadersFromAPIKey(*input.ApiKey, tmpl.HeaderFormat)
		headersBytes, _ := json.Marshal(apiHeaders)
		input.ApiHeaders = string(headersBytes)

		// Re-convert template fields to []map[string]any
		fields = make([]map[string]any, 0, len(input.Fields))
		for _, f := range input.Fields {
			fieldMap := map[string]any{
				"key":          f.Key,
				"label":        f.Label,
				"path":         f.Path,
				"type":         f.Type,
				"format":       f.Format,
				"displayOrder": f.DisplayOrder,
			}
			if f.TotalPath != "" {
				fieldMap["totalPath"] = f.TotalPath
			}
			if f.Unit != "" {
				fieldMap["unit"] = f.Unit
			}
			if len(f.GroupIndex) > 0 {
				fieldMap["groupIndex"] = f.GroupIndex
			}
			if f.Expression != "" {
				fieldMap["expression"] = f.Expression
			}
			fields = append(fields, fieldMap)
		}
	}

	create := client.UsageMonitorChannel.Create().
		SetName(input.Name).
		SetSource(usagemonitorchannel.Source(input.Source)).
		SetAPIURL(input.ApiURL).
		SetAPIMethod(usagemonitorchannel.APIMethod(input.ApiMethod)).
		SetAPIHeaders(apiHeaders).
		SetPollInterval(input.PollInterval).
		SetFields(fields).
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
		existing, err := svc.GetChannel(ctx, id)
		if err == nil && existing.Source == usagemonitorchannel.SourceTemplate {
			tmpl := usage_monitor.GetQuotaMonitorTemplate(string(existing.ProviderType))
			if tmpl != nil {
				apiHeaders := assembleHeadersFromAPIKey(*input.ApiKey, tmpl.HeaderFormat)
				update.SetAPIHeaders(apiHeaders)
			}
		}
	}

	if input.ApiBody != nil {
		update.SetAPIBody(*input.ApiBody)
	}

	if input.PollInterval != nil {
		update.SetPollInterval(*input.PollInterval)
	}

	if input.Fields != nil {
		fields := make([]map[string]any, 0, len(*input.Fields))
		for _, f := range *input.Fields {
			fieldMap := map[string]any{
				"key":          f.Key,
				"label":        f.Label,
				"path":         f.Path,
				"type":         f.Type,
				"format":       f.Format,
				"displayOrder": f.DisplayOrder,
			}
			if f.TotalPath != "" {
				fieldMap["totalPath"] = f.TotalPath
			}
			if f.Unit != "" {
				fieldMap["unit"] = f.Unit
			}
			if len(f.GroupIndex) > 0 {
				fieldMap["groupIndex"] = f.GroupIndex
			}
			if f.Expression != "" {
				fieldMap["expression"] = f.Expression
			}
			fields = append(fields, fieldMap)
		}
		update.SetFields(fields)
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

	// Remove from cache
	svc.cache.Invalidate(id)

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

	apiBody := ""
	if input.ApiBody != nil {
		apiBody = *input.ApiBody
	}

	// Convert []FieldConfig for the checker
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

func (svc *UsageMonitorService) runPollAll(ctx context.Context) {
	all := svc.cache.GetAll()
	if len(all) == 0 {
		return
	}

	log.Debug(ctx, "Polling all usage monitor channels", log.Int("count", len(all)))

	for _, ch := range all {
		svc.pollChannel(ctx, ch)
	}
}

func (svc *UsageMonitorService) pollChannel(ctx context.Context, ch *ent.UsageMonitorChannel) {
	// Convert stored fields back to []FieldConfig
	fieldConfigs := make([]usage_monitor.FieldConfig, 0, len(ch.Fields))
	for _, f := range ch.Fields {
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
			} else if arr, ok := v.([]int); ok {
				fc.GroupIndex = arr
			}
			if v, ok := f["expression"].(string); ok {
				fc.Expression = v
			}
		}
		fieldConfigs = append(fieldConfigs, fc)
	}

	apiBody := ch.APIBody

	pollData, err := svc.genericChecker.Poll(ctx, ch.APIURL, string(ch.APIMethod), ch.APIHeaders, apiBody, fieldConfigs)
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
			"key":     f.Key,
			"label":   f.Label,
			"value":   f.Value,
			"total":   f.Total,
			"percent": f.Percent,
			"unit":    f.Unit,
			"format":  f.Format,
			"error":   f.Error,
		})
	}
	pollDataMap["fields"] = parsedFields

	now := pollData.PolledAt

	// Update DB with success
	client := svc.entFromContext(ctx)
	updated, updateErr := client.UsageMonitorChannel.UpdateOneID(ch.ID).
		SetLastPollData(pollDataMap).
		SetLastPollAt(now).
		SetStatus(usagemonitorchannel.StatusActive).
		ClearLastPollError().
		Save(ctx)
	if updateErr != nil {
		log.Error(ctx, "Failed to update poll data for usage monitor channel",
			log.Int("channel_id", ch.ID),
			log.Cause(updateErr))
	} else {
		// Update cache with the new state
		svc.cache.Set(updated.ID, updated)
	}

	log.Debug(ctx, "Polled usage monitor channel",
		log.Int("channel_id", ch.ID),
		log.String("channel_name", ch.Name),
		log.Int("fields_parsed", len(pollData.Fields)))
}
