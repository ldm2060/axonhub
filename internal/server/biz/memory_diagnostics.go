package biz

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	sampleInterval = 10 * time.Minute
	ringSize       = 144 // 24h at 10-min intervals
	maxGoroutines  = 1000
	topStackCount  = 20
)

type MemorySample struct {
	Timestamp  time.Time `json:"ts"`
	HeapAlloc  uint64    `json:"heapAlloc"`
	HeapInuse  uint64    `json:"heapInuse"`
	Sys        uint64    `json:"sys"`
	Goroutines int       `json:"goroutines"`
	RSSBytes   uint64    `json:"rssBytes"`
}

type MemorySampler struct {
	mu        sync.Mutex
	ring      [ringSize]MemorySample
	pos       int
	count     int
	startTime time.Time
	stopCh    chan struct{}
}

func NewMemorySampler() *MemorySampler {
	return &MemorySampler{
		startTime: time.Now(),
		stopCh:    make(chan struct{}),
	}
}

func (ms *MemorySampler) Run() {
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	// Take initial sample immediately
	ms.sample()

	for {
		select {
		case <-ticker.C:
			ms.sample()
		case <-ms.stopCh:
			return
		}
	}
}

func (ms *MemorySampler) sample() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	sample := MemorySample{
		Timestamp:  time.Now().UTC(),
		HeapAlloc:  m.HeapAlloc,
		HeapInuse:  m.HeapInuse,
		Sys:        m.Sys,
		Goroutines: runtime.NumGoroutine(),
		RSSBytes:   readRSS(),
	}

	ms.mu.Lock()
	ms.ring[ms.pos] = sample
	ms.pos = (ms.pos + 1) % ringSize
	if ms.count < ringSize {
		ms.count++
	}
	ms.mu.Unlock()
}

func (ms *MemorySampler) Stop() {
	close(ms.stopCh)
}

func readRSS() uint64 {
	return readRSSPlatform()
}

type GoroutineStackEntry struct {
	Count int    `json:"count"`
	Stack string `json:"stack"`
}

type MemStatsSnapshot struct {
	HeapAlloc     uint64  `json:"heapAlloc"`
	HeapInuse     uint64  `json:"heapInuse"`
	HeapSys       uint64  `json:"heapSys"`
	HeapIdle      uint64  `json:"heapIdle"`
	HeapReleased  uint64  `json:"heapReleased"`
	HeapObjects   uint64  `json:"heapObjects"`
	StackInuse    uint64  `json:"stackInuse"`
	StackSys      uint64  `json:"stackSys"`
	MspanInuse    uint64  `json:"mspanInuse"`
	MspanSys      uint64  `json:"mspanSys"`
	McacheInuse   uint64  `json:"mcacheInuse"`
	McacheSys     uint64  `json:"mcacheSys"`
	BuckHashSys   uint64  `json:"buckHashSys"`
	GCSys         uint64  `json:"gcSys"`
	OtherSys      uint64  `json:"otherSys"`
	Sys           uint64  `json:"sys"`
	TotalAlloc    uint64  `json:"totalAlloc"`
	Mallocs       uint64  `json:"mallocs"`
	Frees         uint64  `json:"frees"`
	NumGC         uint32  `json:"numGC"`
	GCPauseAvgNs  uint64  `json:"gcPauseAvgNs"`
	GCPauseMaxNs  uint64  `json:"gcPauseMaxNs"`
	GCLastPauseNs uint64  `json:"gcLastPauseNs"`
	GCCPUFraction float64 `json:"gcCpuFraction"`
}

type ProcessInfo struct {
	RSSBytes      uint64  `json:"rssBytes"`
	RSSBytesHuman string  `json:"rssBytesHuman"`
	CPUPercent    float64 `json:"cpuPercent"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
}

type CurrentSnapshot struct {
	MemStats   MemStatsSnapshot       `json:"memStats"`
	Goroutines struct {
		Count     int                   `json:"count"`
		TopStacks []GoroutineStackEntry `json:"topStacks"`
	} `json:"goroutines"`
	Process ProcessInfo `json:"process"`
}

type MemoryDiagnosticsResult struct {
	ExportedAt string          `json:"exportedAt"`
	Current    CurrentSnapshot `json:"current"`
	History    []MemorySample  `json:"history"`
}

func (ms *MemorySampler) Snapshot() *MemoryDiagnosticsResult {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var pauseSum, pauseMax, pauseLast uint64
	numPauses := uint32(256)
	if m.NumGC < numPauses {
		numPauses = m.NumGC
	}
	for i := uint32(0); i < numPauses; i++ {
		idx := (m.NumGC + 256 - 1 - i) % 256
		p := m.PauseNs[idx]
		pauseSum += p
		if p > pauseMax {
			pauseMax = p
		}
	}
	if m.NumGC > 0 {
		pauseLast = m.PauseNs[(m.NumGC+256-1)%256]
	}
	var pauseAvg uint64
	if numPauses > 0 {
		pauseAvg = pauseSum / uint64(numPauses)
	}

	goroutineCount := runtime.NumGoroutine()
	topStacks := collectGoroutineTopStacks()

	rss := readRSS()
	cpu := readCPUPlatform()

	memStats := MemStatsSnapshot{
		HeapAlloc:     m.HeapAlloc,
		HeapInuse:     m.HeapInuse,
		HeapSys:       m.HeapSys,
		HeapIdle:      m.HeapIdle,
		HeapReleased:  m.HeapReleased,
		HeapObjects:   m.HeapObjects,
		StackInuse:    m.StackInuse,
		StackSys:      m.StackSys,
		MspanInuse:    m.MSpanInuse,
		MspanSys:      m.MSpanSys,
		McacheInuse:   m.MCacheInuse,
		McacheSys:     m.MCacheSys,
		BuckHashSys:   m.BuckHashSys,
		GCSys:         m.GCSys,
		OtherSys:      m.OtherSys,
		Sys:           m.Sys,
		TotalAlloc:    m.TotalAlloc,
		Mallocs:       m.Mallocs,
		Frees:         m.Frees,
		NumGC:         m.NumGC,
		GCPauseAvgNs:  pauseAvg,
		GCPauseMaxNs:  pauseMax,
		GCLastPauseNs: pauseLast,
		GCCPUFraction: m.GCCPUFraction,
	}

	current := CurrentSnapshot{
		MemStats: memStats,
		Process: ProcessInfo{
			RSSBytes:      rss,
			RSSBytesHuman: formatBytes(rss),
			CPUPercent:    cpu,
			UptimeSeconds: time.Since(ms.startTime).Seconds(),
		},
	}
	current.Goroutines.Count = goroutineCount
	current.Goroutines.TopStacks = topStacks

	ms.mu.Lock()
	history := make([]MemorySample, 0, ms.count)
	if ms.count < ringSize {
		for i := 0; i < ms.count; i++ {
			history = append(history, ms.ring[i])
		}
	} else {
		for i := 0; i < ringSize; i++ {
			history = append(history, ms.ring[(ms.pos+i)%ringSize])
		}
	}
	ms.mu.Unlock()

	return &MemoryDiagnosticsResult{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Current:   current,
		History:   history,
	}
}

func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func collectGoroutineTopStacks() []GoroutineStackEntry {
	p := pprof.Lookup("goroutine")
	if p == nil {
		return nil
	}

	var buf bytes.Buffer
	if err := p.WriteTo(&buf, 2); err != nil {
		return nil
	}

	stackCount := make(map[string]int)
	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	var currentStack []string
	var inStack bool

	for _, line := range lines {
		lineStr := string(line)
		if bytes.HasPrefix(line, []byte("goroutine ")) {
			if inStack && len(currentStack) > 0 {
				stackKey := strings.Join(currentStack, "\n")
				stackCount[stackKey]++
				if len(stackCount) >= maxGoroutines {
					break
				}
			}
			currentStack = nil
			inStack = true
			continue
		}
		if lineStr == "" {
			if inStack && len(currentStack) > 0 {
				stackKey := strings.Join(currentStack, "\n")
				stackCount[stackKey]++
			}
			currentStack = nil
			inStack = false
			continue
		}
		if inStack {
			currentStack = append(currentStack, lineStr)
		}
	}
	if inStack && len(currentStack) > 0 {
		stackKey := strings.Join(currentStack, "\n")
		stackCount[stackKey]++
	}

	entries := make([]GoroutineStackEntry, 0, len(stackCount))
	for stack, count := range stackCount {
		entries = append(entries, GoroutineStackEntry{Count: count, Stack: stack})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})
	if len(entries) > topStackCount {
		entries = entries[:topStackCount]
	}

	return entries
}
