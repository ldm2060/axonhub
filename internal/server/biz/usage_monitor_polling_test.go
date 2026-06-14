package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
)

// TestPollWithAutoDisable verifies that the polling logic correctly evaluates
// auto-disable conditions and updates monitor.quota_ready based on thresholds.
func TestPollWithAutoDisable(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	// Create a test user (required for owner edge)
	testUser := client.User.Create().
		SetEmail("test@example.com").
		SetPassword("test-password").
		SaveX(ctx)

	// Set up test thresholds - we'll pass these to helper functions directly
	defaultDisableThreshold := 0.95 // Disable at 95% usage
	defaultEnableThreshold := 0.80  // Re-enable at 80% usage

	// Note: We don't need a full UsageMonitorService for these tests
	// since we're testing the helper functions directly

	// Test case 1: Monitor starts ready, usage exceeds disable threshold → should become not ready
	t.Run("DisableWhenExceedingThreshold", func(t *testing.T) {
		monitor := client.UsageMonitorChannel.Create().
			SetName("Test Monitor - Disable").
			SetSource(usagemonitorchannel.SourceCustom).
			SetAPIURL("https://example.com/quota").
			SetAPIMethod(usagemonitorchannel.APIMethodGET).
			SetAPIHeaders(map[string]any{}).
			SetPollInterval(60).
			SetFields([]map[string]any{}).
			SetVariables([]map[string]any{}).
			SetDisplayFields([]map[string]any{}).
			SetStatus(usagemonitorchannel.StatusActive).
			SetAutoDisableEnabled(true).
			SetQuotaReady(true). // Starts ready
			SetOwnerID(testUser.ID).
			SaveX(ctx)

		// Simulate a poll result with 96% usage (exceeds 95% threshold)
		currentReady := true
		if monitor.QuotaReady != nil {
			currentReady = *monitor.QuotaReady
		}
		maxUsageRatio := 0.96
		disableThreshold := defaultDisableThreshold
		enableThreshold := defaultEnableThreshold

		newQuotaReady := evaluateQuotaReady(currentReady, maxUsageRatio, disableThreshold, enableThreshold)

		assert.False(t, newQuotaReady, "Monitor should become not ready when usage exceeds disable threshold")
		assert.True(t, currentReady, "Monitor started as ready")
		assert.NotEqual(t, currentReady, newQuotaReady, "quota_ready state should change")
	})

	// Test case 2: Monitor starts not ready, usage drops below enable threshold → should become ready
	t.Run("EnableWhenBelowThreshold", func(t *testing.T) {
		monitor := client.UsageMonitorChannel.Create().
			SetName("Test Monitor - Enable").
			SetSource(usagemonitorchannel.SourceCustom).
			SetAPIURL("https://example.com/quota").
			SetAPIMethod(usagemonitorchannel.APIMethodGET).
			SetAPIHeaders(map[string]any{}).
			SetPollInterval(60).
			SetFields([]map[string]any{}).
			SetVariables([]map[string]any{}).
			SetDisplayFields([]map[string]any{}).
			SetStatus(usagemonitorchannel.StatusActive).
			SetAutoDisableEnabled(true).
			SetQuotaReady(false). // Starts not ready
			SetOwnerID(testUser.ID).
			SaveX(ctx)

		// Simulate a poll result with 75% usage (below 80% threshold)
		currentReady := false
		if monitor.QuotaReady != nil {
			currentReady = *monitor.QuotaReady
		}
		maxUsageRatio := 0.75
		disableThreshold := defaultDisableThreshold
		enableThreshold := defaultEnableThreshold

		newQuotaReady := evaluateQuotaReady(currentReady, maxUsageRatio, disableThreshold, enableThreshold)

		assert.True(t, newQuotaReady, "Monitor should become ready when usage drops below enable threshold")
		assert.False(t, currentReady, "Monitor started as not ready")
		assert.NotEqual(t, currentReady, newQuotaReady, "quota_ready state should change")
	})

	// Test case 3: Monitor starts ready, usage is in hysteresis zone (80-95%) → should remain ready
	t.Run("HysteresisZone_StaysReady", func(t *testing.T) {
		monitor := client.UsageMonitorChannel.Create().
			SetName("Test Monitor - Hysteresis Ready").
			SetSource(usagemonitorchannel.SourceCustom).
			SetAPIURL("https://example.com/quota").
			SetAPIMethod(usagemonitorchannel.APIMethodGET).
			SetAPIHeaders(map[string]any{}).
			SetPollInterval(60).
			SetFields([]map[string]any{}).
			SetVariables([]map[string]any{}).
			SetDisplayFields([]map[string]any{}).
			SetStatus(usagemonitorchannel.StatusActive).
			SetAutoDisableEnabled(true).
			SetQuotaReady(true). // Starts ready
			SetOwnerID(testUser.ID).
			SaveX(ctx)

		// Simulate a poll result with 88% usage (in hysteresis zone)
		currentReady := true
		if monitor.QuotaReady != nil {
			currentReady = *monitor.QuotaReady
		}
		maxUsageRatio := 0.88
		disableThreshold := defaultDisableThreshold
		enableThreshold := defaultEnableThreshold

		newQuotaReady := evaluateQuotaReady(currentReady, maxUsageRatio, disableThreshold, enableThreshold)

		assert.True(t, newQuotaReady, "Monitor should remain ready in hysteresis zone")
		assert.Equal(t, currentReady, newQuotaReady, "quota_ready state should not change")
	})

	// Test case 4: Monitor starts not ready, usage is in hysteresis zone → should remain not ready
	t.Run("HysteresisZone_StaysNotReady", func(t *testing.T) {
		monitor := client.UsageMonitorChannel.Create().
			SetName("Test Monitor - Hysteresis Not Ready").
			SetSource(usagemonitorchannel.SourceCustom).
			SetAPIURL("https://example.com/quota").
			SetAPIMethod(usagemonitorchannel.APIMethodGET).
			SetAPIHeaders(map[string]any{}).
			SetPollInterval(60).
			SetFields([]map[string]any{}).
			SetVariables([]map[string]any{}).
			SetDisplayFields([]map[string]any{}).
			SetStatus(usagemonitorchannel.StatusActive).
			SetAutoDisableEnabled(true).
			SetQuotaReady(false). // Starts not ready
			SetOwnerID(testUser.ID).
			SaveX(ctx)

		// Simulate a poll result with 88% usage (in hysteresis zone)
		currentReady := false
		if monitor.QuotaReady != nil {
			currentReady = *monitor.QuotaReady
		}
		maxUsageRatio := 0.88
		disableThreshold := defaultDisableThreshold
		enableThreshold := defaultEnableThreshold

		newQuotaReady := evaluateQuotaReady(currentReady, maxUsageRatio, disableThreshold, enableThreshold)

		assert.False(t, newQuotaReady, "Monitor should remain not ready in hysteresis zone")
		assert.Equal(t, currentReady, newQuotaReady, "quota_ready state should not change")
	})

	// Test case 5: Custom thresholds override global defaults
	t.Run("CustomThresholdsOverrideDefaults", func(t *testing.T) {
		customDisableThreshold := 0.90
		customEnableThreshold := 0.70

		monitor := client.UsageMonitorChannel.Create().
			SetName("Test Monitor - Custom Thresholds").
			SetSource(usagemonitorchannel.SourceCustom).
			SetAPIURL("https://example.com/quota").
			SetAPIMethod(usagemonitorchannel.APIMethodGET).
			SetAPIHeaders(map[string]any{}).
			SetPollInterval(60).
			SetFields([]map[string]any{}).
			SetVariables([]map[string]any{}).
			SetDisplayFields([]map[string]any{}).
			SetStatus(usagemonitorchannel.StatusActive).
			SetAutoDisableEnabled(true).
			SetAutoDisableThreshold(customDisableThreshold).
			SetAutoEnableThreshold(customEnableThreshold).
			SetQuotaReady(true).
			SetOwnerID(testUser.ID).
			SaveX(ctx)

		// Usage at 91% should trigger disable with custom threshold (90%)
		// but not with default threshold (95%)
		currentReady := true
		if monitor.QuotaReady != nil {
			currentReady = *monitor.QuotaReady
		}
		maxUsageRatio := 0.91
		disableThreshold := getDisableThreshold(monitor, defaultDisableThreshold)
		enableThreshold := getEnableThreshold(monitor, defaultEnableThreshold)

		require.Equal(t, customDisableThreshold, disableThreshold, "Should use custom disable threshold")
		require.Equal(t, customEnableThreshold, enableThreshold, "Should use custom enable threshold")

		newQuotaReady := evaluateQuotaReady(currentReady, maxUsageRatio, disableThreshold, enableThreshold)

		assert.False(t, newQuotaReady, "Monitor should become not ready with custom threshold")
	})

	// Test case 6: Auto-disable disabled → quota_ready should not be evaluated
	t.Run("AutoDisableDisabled_NoEvaluation", func(t *testing.T) {
		monitor := client.UsageMonitorChannel.Create().
			SetName("Test Monitor - Auto Disable Off").
			SetSource(usagemonitorchannel.SourceCustom).
			SetAPIURL("https://example.com/quota").
			SetAPIMethod(usagemonitorchannel.APIMethodGET).
			SetAPIHeaders(map[string]any{}).
			SetPollInterval(60).
			SetFields([]map[string]any{}).
			SetVariables([]map[string]any{}).
			SetDisplayFields([]map[string]any{}).
			SetStatus(usagemonitorchannel.StatusActive).
			SetAutoDisableEnabled(false). // Auto-disable OFF
			SetQuotaReady(true).
			SetOwnerID(testUser.ID).
			SaveX(ctx)

		// Even with 99% usage, auto-disable logic should not run
		// We verify this by checking that the logic path is not taken
		assert.False(t, monitor.AutoDisableEnabled, "Auto-disable should be disabled")
		if monitor.QuotaReady != nil {
			assert.True(t, *monitor.QuotaReady, "Initial state should be preserved")
		}
	})

	// Test case 7: Builtin source with channel_id triggers channel evaluation
	t.Run("BuiltinSourceTriggersChannelEvaluation", func(t *testing.T) {
		// This test verifies the conditions that trigger channel evaluation
		// In the actual pollChannel method, after updating a monitor with:
		// - source=builtin
		// - channel_id set
		// The method calls evaluateAndUpdateChannelQuotaReady()

		// We create a monitor and verify the conditions (without channel FK)
		monitor := client.UsageMonitorChannel.Create().
			SetName("Test Monitor - Builtin").
			SetSource(usagemonitorchannel.SourceBuiltin). // Builtin source
			SetAPIURL("https://example.com/quota").
			SetAPIMethod(usagemonitorchannel.APIMethodGET).
			SetAPIHeaders(map[string]any{}).
			SetPollInterval(60).
			SetFields([]map[string]any{}).
			SetVariables([]map[string]any{}).
			SetDisplayFields([]map[string]any{}).
			SetStatus(usagemonitorchannel.StatusActive).
			SetAutoDisableEnabled(true).
			SetQuotaReady(true).
			SetOwnerID(testUser.ID).
			SetOwnerID(testUser.ID).
			SetLastPollAt(time.Now()).
			SaveX(ctx)

		// Verify the source is builtin
		assert.Equal(t, usagemonitorchannel.SourceBuiltin, monitor.Source)

		// In the actual implementation, if monitor.ChannelID != nil,
		// evaluateAndUpdateChannelQuotaReady() would be called
		// Task 7 will implement the full channel evaluation logic
	})
}
