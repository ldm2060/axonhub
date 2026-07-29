package biz

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSystemRuntimeMonitorCalculatesRatesAndStats(t *testing.T) {
	base := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	now := base
	raws := []systemRuntimeRawMetrics{
		{
			SystemCPUIdle:      100,
			SystemCPUTotal:     200,
			ProcessCPUNanos:    1_000_000_000,
			MemoryUsedBytes:    40,
			MemoryTotalBytes:   100,
			NetworkReceived:    1_000,
			NetworkTransmitted: 2_000,
			DiskUsedBytes:      50,
			DiskTotalBytes:     100,
			ProcessRSSBytes:    1_024,
		},
		{
			SystemCPUIdle:      120,
			SystemCPUTotal:     300,
			ProcessCPUNanos:    1_000_000_000 + uint64(5*time.Second)*uint64(max(1, runtime.NumCPU()))/2,
			MemoryUsedBytes:    60,
			MemoryTotalBytes:   100,
			NetworkReceived:    1_500,
			NetworkTransmitted: 3_000,
			DiskUsedBytes:      75,
			DiskTotalBytes:     100,
			ProcessRSSBytes:    2_048,
		},
	}
	index := 0
	monitor := newSystemRuntimeMonitor(func() systemRuntimeRawMetrics {
		value := raws[index]
		index++
		return value
	}, func() time.Time { return now })

	monitor.sample()
	now = now.Add(5 * time.Second)
	monitor.sample()

	overview := monitor.Overview()
	require.Len(t, overview.History, 2)
	require.InDelta(t, 80, overview.Current.SystemCPUPercent, 0.001)
	require.InDelta(t, 50, overview.Current.ProcessCPUPercent, 0.001)
	require.InDelta(t, 60, overview.Current.MemoryUsedPercent, 0.001)
	require.InDelta(t, 100, overview.Current.NetworkReceiveBytesPerSec, 0.001)
	require.InDelta(t, 200, overview.Current.NetworkTransmitBytesPerSec, 0.001)
	require.InDelta(t, 75, overview.Current.DiskUsedPercent, 0.001)
	require.Equal(t, 2, overview.Stats.SampleCount)
	require.InDelta(t, 40, overview.Stats.SystemCPUAveragePercent, 0.001)
	require.InDelta(t, 80, overview.Stats.SystemCPUMaxPercent, 0.001)
	require.Equal(t, uint64(2_048), overview.Stats.ProcessRSSMaxBytes)
}

func TestSystemRuntimeMonitorRetainsOnlyThreeHours(t *testing.T) {
	now := time.Date(2026, time.July, 27, 8, 0, 0, 0, time.UTC)
	monitor := newSystemRuntimeMonitor(func() systemRuntimeRawMetrics {
		return systemRuntimeRawMetrics{MemoryUsedBytes: 1, MemoryTotalBytes: 2}
	}, func() time.Time { return now })

	monitor.sample()
	now = now.Add(runtimeRetention + time.Second)
	monitor.sample()

	overview := monitor.Overview()
	require.Len(t, overview.History, 1)
	require.Equal(t, now, overview.Current.Timestamp)
}
