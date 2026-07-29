package biz

import (
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/ldm2060/axonhub/internal/build"
)

const (
	runtimeSampleInterval = 5 * time.Second
	runtimeRetention      = 3 * time.Hour
)

type systemRuntimeRawMetrics struct {
	SystemCPUIdle      uint64
	SystemCPUTotal     uint64
	ProcessCPUNanos    uint64
	MemoryUsedBytes    uint64
	MemoryTotalBytes   uint64
	NetworkReceived    uint64
	NetworkTransmitted uint64
	DiskUsedBytes      uint64
	DiskTotalBytes     uint64
	Load1              float64
	Load5              float64
	Load15             float64
	ProcessRSSBytes    uint64
	ProcessThreads     int
}

type systemRuntimeHostDetails struct {
	Platform        string
	PlatformVersion string
	KernelVersion   string
}

type SystemRuntimeHost struct {
	Hostname         string    `json:"hostname"`
	OS               string    `json:"os"`
	Architecture     string    `json:"architecture"`
	Platform         string    `json:"platform"`
	PlatformVersion  string    `json:"platformVersion"`
	KernelVersion    string    `json:"kernelVersion"`
	LogicalCPUs      int       `json:"logicalCpus"`
	ProcessID        int       `json:"processId"`
	GoVersion        string    `json:"goVersion"`
	Version          string    `json:"version"`
	ServiceStartedAt time.Time `json:"serviceStartedAt"`
}

type SystemRuntimeSample struct {
	Timestamp                  time.Time `json:"timestamp"`
	SystemCPUPercent           float64   `json:"systemCpuPercent"`
	ProcessCPUPercent          float64   `json:"processCpuPercent"`
	MemoryUsedPercent          float64   `json:"memoryUsedPercent"`
	MemoryUsedBytes            uint64    `json:"memoryUsedBytes"`
	MemoryTotalBytes           uint64    `json:"memoryTotalBytes"`
	ProcessRSSBytes            uint64    `json:"processRssBytes"`
	ProcessHeapAllocBytes      uint64    `json:"processHeapAllocBytes"`
	NetworkReceiveBytesPerSec  float64   `json:"networkReceiveBytesPerSecond"`
	NetworkTransmitBytesPerSec float64   `json:"networkTransmitBytesPerSecond"`
	DiskUsedPercent            float64   `json:"diskUsedPercent"`
	DiskUsedBytes              uint64    `json:"diskUsedBytes"`
	DiskTotalBytes             uint64    `json:"diskTotalBytes"`
	Load1                      float64   `json:"load1"`
	Load5                      float64   `json:"load5"`
	Load15                     float64   `json:"load15"`
	Goroutines                 int       `json:"goroutines"`
	ProcessThreads             int       `json:"processThreads"`
	GCCount                    uint32    `json:"gcCount"`
	GCPauseMilliseconds        float64   `json:"gcPauseMilliseconds"`
	ServiceUptimeSeconds       float64   `json:"serviceUptimeSeconds"`
}

type SystemRuntimeStats struct {
	PeriodStart                       time.Time `json:"periodStart"`
	PeriodEnd                         time.Time `json:"periodEnd"`
	SampleCount                       int       `json:"sampleCount"`
	SystemCPUAveragePercent           float64   `json:"systemCpuAveragePercent"`
	SystemCPUMaxPercent               float64   `json:"systemCpuMaxPercent"`
	ProcessCPUAveragePercent          float64   `json:"processCpuAveragePercent"`
	ProcessCPUMaxPercent              float64   `json:"processCpuMaxPercent"`
	MemoryUsedAveragePercent          float64   `json:"memoryUsedAveragePercent"`
	MemoryUsedMaxPercent              float64   `json:"memoryUsedMaxPercent"`
	ProcessRSSAverageBytes            float64   `json:"processRssAverageBytes"`
	ProcessRSSMaxBytes                uint64    `json:"processRssMaxBytes"`
	NetworkReceiveTotalBytes          float64   `json:"networkReceiveTotalBytes"`
	NetworkTransmitTotalBytes         float64   `json:"networkTransmitTotalBytes"`
	NetworkReceivePeakBytesPerSecond  float64   `json:"networkReceivePeakBytesPerSecond"`
	NetworkTransmitPeakBytesPerSecond float64   `json:"networkTransmitPeakBytesPerSecond"`
}

type SystemRuntimeOverview struct {
	CollectedAt    time.Time             `json:"collectedAt"`
	SampleInterval int                   `json:"sampleIntervalSeconds"`
	Retention      int                   `json:"retentionSeconds"`
	Host           SystemRuntimeHost     `json:"host"`
	Current        SystemRuntimeSample   `json:"current"`
	Stats          SystemRuntimeStats    `json:"stats"`
	History        []SystemRuntimeSample `json:"history"`
}

type SystemRuntimeMonitor struct {
	mu          sync.Mutex
	startedAt   time.Time
	host        SystemRuntimeHost
	samples     []SystemRuntimeSample
	previousRaw systemRuntimeRawMetrics
	previousAt  time.Time
	hasPrevious bool
	collect     func() systemRuntimeRawMetrics
	now         func() time.Time
}

func NewSystemRuntimeMonitor() *SystemRuntimeMonitor {
	return newSystemRuntimeMonitor(collectSystemRuntimeMetrics, time.Now)
}

func newSystemRuntimeMonitor(collect func() systemRuntimeRawMetrics, now func() time.Time) *SystemRuntimeMonitor {
	startedAt := now()
	hostname, _ := os.Hostname()
	platform := collectSystemRuntimeHostDetails()
	info := build.GetBuildInfo()
	return &SystemRuntimeMonitor{
		startedAt: startedAt,
		host: SystemRuntimeHost{
			Hostname:         hostname,
			OS:               runtime.GOOS,
			Architecture:     runtime.GOARCH,
			Platform:         platform.Platform,
			PlatformVersion:  platform.PlatformVersion,
			KernelVersion:    platform.KernelVersion,
			LogicalCPUs:      runtime.NumCPU(),
			ProcessID:        os.Getpid(),
			GoVersion:        runtime.Version(),
			Version:          info.Version,
			ServiceStartedAt: startedAt,
		},
		collect: collect,
		now:     now,
	}
}

func (m *SystemRuntimeMonitor) sample() {
	now := m.now().UTC()
	raw := m.collect()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	sample := SystemRuntimeSample{
		Timestamp:             now,
		MemoryUsedBytes:       raw.MemoryUsedBytes,
		MemoryTotalBytes:      raw.MemoryTotalBytes,
		ProcessRSSBytes:       raw.ProcessRSSBytes,
		ProcessHeapAllocBytes: mem.HeapAlloc,
		DiskUsedBytes:         raw.DiskUsedBytes,
		DiskTotalBytes:        raw.DiskTotalBytes,
		Load1:                 raw.Load1,
		Load5:                 raw.Load5,
		Load15:                raw.Load15,
		Goroutines:            runtime.NumGoroutine(),
		ProcessThreads:        raw.ProcessThreads,
		GCCount:               mem.NumGC,
		ServiceUptimeSeconds:  now.Sub(m.startedAt).Seconds(),
	}
	if raw.MemoryTotalBytes > 0 {
		sample.MemoryUsedPercent = percentage(raw.MemoryUsedBytes, raw.MemoryTotalBytes)
	}
	if raw.DiskTotalBytes > 0 {
		sample.DiskUsedPercent = percentage(raw.DiskUsedBytes, raw.DiskTotalBytes)
	}
	if mem.NumGC > 0 {
		sample.GCPauseMilliseconds = float64(mem.PauseNs[(mem.NumGC+255)%256]) / float64(time.Millisecond)
	}

	m.mu.Lock()
	if m.hasPrevious {
		elapsed := now.Sub(m.previousAt).Seconds()
		if elapsed > 0 {
			if raw.SystemCPUTotal >= m.previousRaw.SystemCPUTotal && raw.SystemCPUIdle >= m.previousRaw.SystemCPUIdle {
				totalDelta := raw.SystemCPUTotal - m.previousRaw.SystemCPUTotal
				idleDelta := raw.SystemCPUIdle - m.previousRaw.SystemCPUIdle
				if totalDelta > 0 && idleDelta <= totalDelta {
					sample.SystemCPUPercent = clampPercent(float64(totalDelta-idleDelta) / float64(totalDelta) * 100)
				}
			}
			if raw.ProcessCPUNanos >= m.previousRaw.ProcessCPUNanos {
				sample.ProcessCPUPercent = clampPercent(float64(raw.ProcessCPUNanos-m.previousRaw.ProcessCPUNanos) / (elapsed * float64(time.Second)) / float64(max(1, runtime.NumCPU())) * 100)
			}
			if raw.NetworkReceived >= m.previousRaw.NetworkReceived {
				sample.NetworkReceiveBytesPerSec = float64(raw.NetworkReceived-m.previousRaw.NetworkReceived) / elapsed
			}
			if raw.NetworkTransmitted >= m.previousRaw.NetworkTransmitted {
				sample.NetworkTransmitBytesPerSec = float64(raw.NetworkTransmitted-m.previousRaw.NetworkTransmitted) / elapsed
			}
		}
	}
	m.previousRaw = raw
	m.previousAt = now
	m.hasPrevious = true
	m.samples = append(m.samples, sample)
	cutoff := now.Add(-runtimeRetention)
	first := 0
	for first < len(m.samples) && m.samples[first].Timestamp.Before(cutoff) {
		first++
	}
	if first > 0 {
		m.samples = append([]SystemRuntimeSample(nil), m.samples[first:]...)
	}
	m.mu.Unlock()
}

func (m *SystemRuntimeMonitor) Overview() SystemRuntimeOverview {
	m.mu.Lock()
	history := append([]SystemRuntimeSample(nil), m.samples...)
	host := m.host
	m.mu.Unlock()

	if len(history) == 0 {
		m.sample()
		m.mu.Lock()
		history = append([]SystemRuntimeSample(nil), m.samples...)
		m.mu.Unlock()
	}
	current := history[len(history)-1]
	return SystemRuntimeOverview{
		CollectedAt:    time.Now().UTC(),
		SampleInterval: int(runtimeSampleInterval / time.Second),
		Retention:      int(runtimeRetention / time.Second),
		Host:           host,
		Current:        current,
		Stats:          calculateSystemRuntimeStats(history),
		History:        history,
	}
}

func calculateSystemRuntimeStats(samples []SystemRuntimeSample) SystemRuntimeStats {
	stats := SystemRuntimeStats{SampleCount: len(samples)}
	if len(samples) == 0 {
		return stats
	}
	stats.PeriodStart = samples[0].Timestamp
	stats.PeriodEnd = samples[len(samples)-1].Timestamp
	for _, sample := range samples {
		stats.SystemCPUAveragePercent += sample.SystemCPUPercent
		stats.ProcessCPUAveragePercent += sample.ProcessCPUPercent
		stats.MemoryUsedAveragePercent += sample.MemoryUsedPercent
		stats.ProcessRSSAverageBytes += float64(sample.ProcessRSSBytes)
		stats.SystemCPUMaxPercent = math.Max(stats.SystemCPUMaxPercent, sample.SystemCPUPercent)
		stats.ProcessCPUMaxPercent = math.Max(stats.ProcessCPUMaxPercent, sample.ProcessCPUPercent)
		stats.MemoryUsedMaxPercent = math.Max(stats.MemoryUsedMaxPercent, sample.MemoryUsedPercent)
		stats.ProcessRSSMaxBytes = max(stats.ProcessRSSMaxBytes, sample.ProcessRSSBytes)
		stats.NetworkReceiveTotalBytes += sample.NetworkReceiveBytesPerSec * runtimeSampleInterval.Seconds()
		stats.NetworkTransmitTotalBytes += sample.NetworkTransmitBytesPerSec * runtimeSampleInterval.Seconds()
		stats.NetworkReceivePeakBytesPerSecond = math.Max(stats.NetworkReceivePeakBytesPerSecond, sample.NetworkReceiveBytesPerSec)
		stats.NetworkTransmitPeakBytesPerSecond = math.Max(stats.NetworkTransmitPeakBytesPerSecond, sample.NetworkTransmitBytesPerSec)
	}
	count := float64(len(samples))
	stats.SystemCPUAveragePercent /= count
	stats.ProcessCPUAveragePercent /= count
	stats.MemoryUsedAveragePercent /= count
	stats.ProcessRSSAverageBytes /= count
	return stats
}

func percentage(used, total uint64) float64 {
	return clampPercent(float64(used) / float64(total) * 100)
}

func clampPercent(value float64) float64 {
	return math.Max(0, math.Min(100, value))
}
