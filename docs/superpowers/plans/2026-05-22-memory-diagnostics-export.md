# Memory Diagnostics Export — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a one-click "Export Memory Diagnostics" button in the System → Diagnostics tab that downloads a JSON file with current runtime memory stats + goroutine top stacks + 24h trend of lightweight samples.

**Architecture:** Backend `MemorySampler` runs a 10-min ticker goroutine filling a 144-slot ring buffer with `runtime.MemStats` summaries + RSS. On export request, it collects a full snapshot (MemStats + goroutine stacks + process info) and merges with ring buffer history. GraphQL `getMemoryDiagnostics` reuses `CacheDiagnosticsPayload` type. Frontend adds a parallel button in `diagnostics-settings.tsx` following the existing cache diagnostics pattern.

**Tech Stack:** Go runtime/pprof + syscall (backend), GraphQL/gqlgen (API), React/TanStack Query (frontend), i18next (i18n)

---

### Task 1: Backend — MemorySampler core (ring buffer + sampling goroutine)

**Files:**
- Create: `internal/server/biz/memory_diagnostics.go`

- [ ] **Step 1: Create `memory_diagnostics.go` with MemorySampler struct, MemorySample, and NewMemorySampler**

```go
package biz

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/pprof"
	"sort"
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
```

- [ ] **Step 2: Add `run()` method — the background sampling goroutine**

```go
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
```

- [ ] **Step 3: Add `Stop()` method**

```go
func (ms *MemorySampler) Stop() {
	close(ms.stopCh)
}
```

- [ ] **Step 4: Add `readRSS()` — cross-platform process RSS reader**

```go
func readRSS() uint64 {
	// Platform-specific implementations are in memory_diagnostics_platform.go
	return readRSSPlatform()
}
```

- [ ] **Step 5: Create `memory_diagnostics_platform_linux.go`**

```go
//go:build linux

package biz

import (
	"os"
	"strconv"
	"strings"
)

func readRSSPlatform() uint64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return kb * 1024
				}
			}
		}
	}
	return 0
}

func readCPUPlatform() float64 {
	// CPU estimation on Linux is complex; return 0 for now.
	// The Go runtime's gcCpuFraction provides an approximation.
	return 0
}
```

- [ ] **Step 6: Create `memory_diagnostics_platform_windows.go`**

```go
//go:build windows

package biz

import (
	"syscall"
	"unsafe"
)

var (
	modpsapi                = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo = modpsapi.NewProc("GetProcessMemoryInfo")
)

type PROCESS_MEMORY_COUNTERS struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

func readRSSPlatform() uint64 {
	process := syscall.CurrentProcess()
	var counters PROCESS_MEMORY_COUNTERS
	counters.CB = uint32(unsafe.Sizeof(counters))

	ret, _, _ := procGetProcessMemoryInfo.Call(
		process,
		uintptr(unsafe.Pointer(&counters)),
		uintptr(unsafe.Sizeof(counters)),
	)
	if ret == 0 {
		return 0
	}
	return uint64(counters.WorkingSetSize)
}

func readCPUPlatform() float64 {
	return 0
}
```

- [ ] **Step 7: Create `memory_diagnostics_platform_other.go` (fallback for macOS etc.)**

```go
//go:build !linux && !windows

package biz

import "runtime"

func readRSSPlatform() uint64 {
	// Fallback: use MemStats.Sys as rough approximation
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys
}

func readCPUPlatform() float64 {
	return 0
}
```

- [ ] **Step 8: Add `Snapshot()` method — the full export data collector**

```go
type GoroutineStackEntry struct {
	Count int    `json:"count"`
	Stack string `json:"stack"`
}

type MemStatsSnapshot struct {
	HeapAlloc      uint64  `json:"heapAlloc"`
	HeapInuse      uint64  `json:"heapInuse"`
	HeapSys        uint64  `json:"heapSys"`
	HeapIdle       uint64  `json:"heapIdle"`
	HeapReleased   uint64  `json:"heapReleased"`
	HeapObjects    uint64  `json:"heapObjects"`
	StackInuse     uint64  `json:"stackInuse"`
	StackSys       uint64  `json:"stackSys"`
	MspanInuse     uint64  `json:"mspanInuse"`
	MspanSys       uint64  `json:"mspanSys"`
	McacheInuse    uint64  `json:"mcacheInuse"`
	McacheSys      uint64  `json:"mcacheSys"`
	BuckHashSys    uint64  `json:"buckHashSys"`
	GCSys          uint64  `json:"gcSys"`
	OtherSys       uint64  `json:"otherSys"`
	Sys            uint64  `json:"sys"`
	TotalAlloc     uint64  `json:"totalAlloc"`
	Mallocs        uint64  `json:"mallocs"`
	Frees          uint64  `json:"frees"`
	NumGC          uint32  `json:"numGC"`
	GCPauseAvgNs   uint64  `json:"gcPauseAvgNs"`
	GCPauseMaxNs   uint64  `json:"gcPauseMaxNs"`
	GCLastPauseNs  uint64  `json:"gcLastPauseNs"`
	GCCPUFraction  float64 `json:"gcCpuFraction"`
}

type ProcessInfo struct {
	RSSBytes      uint64  `json:"rssBytes"`
	RSSBytesHuman string  `json:"rssBytesHuman"`
	CPUPercent    float64 `json:"cpuPercent"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
}

type CurrentSnapshot struct {
	MemStats   MemStatsSnapshot      `json:"memStats"`
	Goroutines struct {
		Count     int                  `json:"count"`
		TopStacks []GoroutineStackEntry `json:"topStacks"`
	} `json:"goroutines"`
	Process    ProcessInfo           `json:"process"`
}

type MemoryDiagnosticsResult struct {
	ExportedAt string          `json:"exportedAt"`
	Current    CurrentSnapshot `json:"current"`
	History    []MemorySample  `json:"history"`
}

func (ms *MemorySampler) Snapshot() *MemoryDiagnosticsResult {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Compute GC pause stats from the circular buffer of recent pauses
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

	// Collect goroutine stacks
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
		MspanInuse:    m.MspanInuse,
		MspanSys:      m.MspanSys,
		McacheInuse:   m.McacheInuse,
		McacheSys:     m.McacheSys,
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

	// Copy ring buffer history in chronological order
	ms.mu.Lock()
	history := make([]MemorySample, 0, ms.count)
	if ms.count < ringSize {
		// Buffer not yet full — entries 0..count-1 in order
		for i := 0; i < ms.count; i++ {
			history = append(history, ms.ring[i])
		}
	} else {
		// Buffer full — pos is the next overwrite slot, so oldest is at pos
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
	// debug=2 gives full stacks for each goroutine
	if err := p.WriteTo(&buf, 2); err != nil {
		return nil
	}

	// Parse the output: each goroutine block starts with "goroutine N [state]:"
	// followed by its stack trace lines
	stackCount := make(map[string]int)
	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	var currentStack []string
	var inStack bool

	for _, line := lines {
		lineStr := string(line)
		if bytes.HasPrefix(line, []byte("goroutine ")) {
			// New goroutine block — flush previous stack
			if inStack && len(currentStack) > 0 {
				stackKey := strings.Join(currentStack, "\n")
				stackCount[stackKey]++
				if len(stackCount) >= maxGoroutines {
					break // safety cap
				}
			}
			currentStack = nil
			inStack = true
			continue
		}
		if lineStr == "" {
			// End of goroutine block
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
	// Flush last stack if file didn't end with empty line
	if inStack && len(currentStack) > 0 {
		stackKey := strings.Join(currentStack, "\n")
		stackCount[stackKey]++
	}

	// Sort by count descending, take top 20
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
```

- [ ] **Step 9: Commit**

```bash
git add internal/server/biz/memory_diagnostics.go internal/server/biz/memory_diagnostics_platform_linux.go internal/server/biz/memory_diagnostics_platform_windows.go internal/server/biz/memory_diagnostics_platform_other.go
git commit -m "feat: add MemorySampler with ring buffer and goroutine stack collection"
```

---

### Task 2: Backend — fx wiring (register MemorySampler + lifecycle hooks)

**Files:**
- Modify: `internal/server/biz/fx_module.go`

- [ ] **Step 1: Add MemorySampler provider and lifecycle hooks to fx_module.go**

Add `fx.Provide(NewMemorySampler)` after the existing providers, and add an `fx.Invoke` block for lifecycle (start the sampler goroutine on OnStart, stop on OnStop):

```go
// In the Module var, after fx.Provide(NewEmailService):
fx.Provide(NewMemorySampler),

// Add lifecycle hook after the ProviderQuotaService Invoke block:
fx.Invoke(func(lc fx.Lifecycle, sampler *MemorySampler) {
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            go sampler.Run()
            return nil
        },
        OnStop: func(ctx context.Context) error {
            sampler.Stop()
            return nil
        },
    })
}),
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/biz/fx_module.go
git commit -m "feat: register MemorySampler in fx module with lifecycle hooks"
```

---

### Task 3: Backend — GraphQL schema + resolver + wiring

**Files:**
- Modify: `internal/server/gql/system.graphql` — add `getMemoryDiagnostics` query
- Modify: `internal/server/gql/system.resolvers.go` — add resolver method
- Modify: `internal/server/gql/resolver.go` — add `memorySampler` field to Resolver + NewSchema
- Modify: `internal/server/gql/graphql.go` — add MemorySampler to Dependencies + NewSchema call
- Modify (generated): `internal/server/gql/generated.go`, `models_gen.go` — via `go generate`

- [ ] **Step 1: Add GraphQL schema entry to system.graphql**

In `internal/server/gql/system.graphql`, in the `extend type Query` block that contains `getCacheDiagnostics`, add:

```graphql
  getMemoryDiagnostics: GetCacheDiagnosticsPayload!
```

Note: We reuse `GetCacheDiagnosticsPayload` type (which has `fileName`, `content`, `targets` fields). The `targets` field will be empty for memory diagnostics.

- [ ] **Step 2: Run gqlgen code generation**

```bash
cd internal/server/gql && go generate ./...
```

This regenerates `generated.go` and `models_gen.go`, and will add a stub `GetMemoryDiagnostics` method to `system.resolvers.go` (if not already present).

- [ ] **Step 3: Add `memorySampler` field to Resolver struct in resolver.go**

In `internal/server/gql/resolver.go`, add field to `Resolver` struct:

```go
memorySampler *biz.MemorySampler
```

Add parameter to `NewSchema` function signature (after `videoWorker`):

```go
memorySampler *biz.MemorySampler,
```

And in the `&Resolver{...}` initialization:

```go
memorySampler: memorySampler,
```

- [ ] **Step 4: Add MemorySampler to Dependencies struct in graphql.go**

In `internal/server/gql/graphql.go`, add to `Dependencies` struct:

```go
MemorySampler *biz.MemorySampler
```

And add `deps.MemorySampler` to the `NewSchema(...)` call arguments (after `deps.VideoWorker`).

- [ ] **Step 5: Add `GetMemoryDiagnostics` resolver method in system.resolvers.go**

```go
// GetMemoryDiagnostics is the resolver for the getMemoryDiagnostics field.
func (r *queryResolver) GetMemoryDiagnostics(ctx context.Context) (*GetCacheDiagnosticsPayload, error) {
    user, ok := contexts.GetUser(ctx)
    if !ok || user == nil || !user.IsOwner {
        return nil, ErrNotOwner
    }

    result := r.memorySampler.Snapshot()

    content, err := json.MarshalIndent(result, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("failed to marshal memory diagnostics: %w", err)
    }

    return &GetCacheDiagnosticsPayload{
        FileName: fmt.Sprintf("memory-diagnostics-%s.json", time.Now().UTC().Format("20060102T150405Z")),
        Content:  string(content),
        Targets:  nil,
    }, nil
}
```

Add `"encoding/json"` to the imports in `system.resolvers.go` if not already present.

- [ ] **Step 6: Commit**

```bash
git add internal/server/gql/system.graphql internal/server/gql/system.resolvers.go internal/server/gql/resolver.go internal/server/gql/graphql.go internal/server/gql/generated.go internal/server/gql/models_gen.go
git commit -m "feat: add getMemoryDiagnostics GraphQL query and resolver"
```

---

### Task 4: Frontend — data layer (query hook)

**Files:**
- Modify: `frontend/src/features/system/data/system.ts`

- [ ] **Step 1: Add GraphQL query constant and export types**

Add after the existing `GET_CACHE_DIAGNOSTICS_QUERY`:

```typescript
const GET_MEMORY_DIAGNOSTICS_QUERY = `
  query GetMemoryDiagnostics {
    getMemoryDiagnostics {
      fileName
      content
      targets
    }
  }
`;
```

- [ ] **Step 2: Add `useExportMemoryDiagnostics` hook**

Add after the existing `useExportCacheDiagnostics` function:

```typescript
export function useExportMemoryDiagnostics() {
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async () => {
      const data = await graphqlRequest<{ getMemoryDiagnostics: GetCacheDiagnosticsPayload }>(
        GET_MEMORY_DIAGNOSTICS_QUERY
      );
      return data.getMemoryDiagnostics;
    },
    onSuccess: (data) => {
      const blob = new Blob([data.content], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = data.fileName;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      toast.success(i18n.t('system.memory_diagnostics.success'));
    },
    onError: (error) => {
      handleError(error, i18n.t('system.memory_diagnostics.error'));
    },
  });
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/system/data/system.ts
git commit -m "feat: add useExportMemoryDiagnostics query hook"
```

---

### Task 5: Frontend — UI button in diagnostics-settings.tsx

**Files:**
- Modify: `frontend/src/features/system/components/diagnostics-settings.tsx`

- [ ] **Step 1: Add import for the new hook**

Add to the existing import from `../data/system`:

```typescript
import { useClearCache, useExportCacheDiagnostics, useExportMemoryDiagnostics } from '../data/system';
```

- [ ] **Step 2: Add import for `Download` icon**

Add `Download` to the existing lucide-react import:

```typescript
import { Download, RefreshCw } from 'lucide-react';
```

- [ ] **Step 3: Add the hook call and button in DiagnosticsSettings**

Add `useExportMemoryDiagnostics` hook call alongside the existing hooks:

```typescript
const { mutate: exportMemoryDiagnostics, isPending: isExportingMemory } = useExportMemoryDiagnostics();
```

Add a new `<div className='rounded-lg border p-3 sm:p-4'>` block after the cache diagnostics block (inside `<CardContent>`), with the same layout pattern:

```tsx
<div className='rounded-lg border p-3 sm:p-4'>
  <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4'>
    <div className='space-y-1 min-w-0'>
      <h4 className='text-sm font-medium'>{t('system.memory_diagnostics.title')}</h4>
      <p className='text-muted-foreground text-sm'>{t('system.memory_diagnostics.description')}</p>
    </div>
    <Button variant='outline' size='sm' onClick={() => exportMemoryDiagnostics()} disabled={isExportingMemory} className='w-full sm:w-auto'>
      <Download className={`mr-2 h-4 w-4 ${isExportingMemory ? 'animate-spin' : ''}`} />
      {t('system.memory_diagnostics.export')}
    </Button>
  </div>
</div>
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/features/system/components/diagnostics-settings.tsx
git commit -m "feat: add Export Memory Diagnostics button in diagnostics settings"
```

---

### Task 6: Frontend — i18n keys

**Files:**
- Modify: `frontend/src/locales/en/system.json`
- Modify: `frontend/src/locales/zh-CN/system.json`

- [ ] **Step 1: Add English i18n keys**

Add these keys to `frontend/src/locales/en/system.json` (after the existing `system.diagnostics.cache.clearFailed` key):

```json
"system.memory_diagnostics.title": "Memory Diagnostics",
"system.memory_diagnostics.description": "Export current memory usage, goroutine summary, and 24h trend data for leak analysis.",
"system.memory_diagnostics.export": "Export Memory Diagnostics",
"system.memory_diagnostics.success": "Memory diagnostics exported successfully",
"system.memory_diagnostics.error": "Failed to export memory diagnostics",
```

- [ ] **Step 2: Add Chinese i18n keys**

Add these keys to `frontend/src/locales/zh-CN/system.json` (after the existing `system.diagnostics.cache.clearFailed` key):

```json
"system.memory_diagnostics.title": "内存诊断",
"system.memory_diagnostics.description": "导出当前内存使用情况、Goroutine 摘要及 24 小时趋势数据，用于排查内存泄漏。",
"system.memory_diagnostics.export": "导出内存诊断",
"system.memory_diagnostics.success": "内存诊断导出成功",
"system.memory_diagnostics.error": "内存诊断导出失败",
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/locales/en/system.json frontend/src/locales/zh-CN/system.json
git commit -m "feat: add i18n keys for memory diagnostics (en + zh-CN)"
```

---

### Task 7: Build verification

- [ ] **Step 1: Verify Go backend compiles**

```bash
go build ./...
```

Expected: compiles without errors.

- [ ] **Step 2: Verify frontend compiles**

```bash
cd frontend && npm run build
```

Expected: builds without type errors.

- [ ] **Step 3: Commit (if any fix-ups were needed)**

Only commit if build issues required fixes. Otherwise skip this step.