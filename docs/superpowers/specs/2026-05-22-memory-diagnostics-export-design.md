# Memory Diagnostics Export — Design Spec

## Goal

Add a "Export Memory Diagnostics" button to the existing System → Diagnostics tab (owner-only), allowing one-click download of a JSON file containing the current memory snapshot plus a 24h trend of lightweight samples. This replaces the need for manually enabling pprof and running `go tool pprof` locally — the owner can just click a button, download a file, and share it for analysis.

## Approach

Follow the exact same pattern as `getCacheDiagnostics`: a GraphQL query returning `CacheDiagnosticsPayload` (fileName + content + targets), with the `content` field being a JSON string. The frontend reuses the existing Blob-download logic.

## Exported JSON Structure

Filename: `memory-diagnostics-20260522-153000.json`

```json
{
  "exportedAt": "2026-05-22T15:30:00+08:00",
  "current": {
    "memStats": {
      "heapAlloc":      134217728,
      "heapInuse":      150994944,
      "heapSys":        201326592,
      "heapIdle":       50331648,
      "heapReleased":   41943040,
      "heapObjects":    876543,
      "stackInuse":     8388608,
      "stackSys":       8388608,
      "mspanInuse":     3145728,
      "mspanSys":       4194304,
      "mcacheInuse":    13800,
      "mcacheSys":      16384,
      "buckHashSys":    1441792,
      "gcSys":          9437184,
      "otherSys":       327680,
      "sys":            440401920,
      "totalAlloc":     9876543210,
      "mallocs":        54321098,
      "frees":          53444555,
      "numGC":          42,
      "gcPauseAvgNs":   12345,
      "gcPauseMaxNs":   987654,
      "gcLastPauseNs":  54321,
      "gcCpuFraction":  0.00012
    },
    "goroutines": {
      "count": 127,
      "topStacks": [
        {"count": 23, "stack": "net/http.(*conn).serve\nnet/http.serverHandler.ServeHTTP\n..."}
      ]
    },
    "process": {
      "rssBytes":      524288000,
      "rssBytesHuman": "500.0 MB",
      "cpuPercent":    2.3,
      "uptimeSeconds": 86400
    }
  },
  "history": [
    {
      "ts":          "2026-05-21T15:30:00+08:00",
      "heapAlloc":   128000000,
      "heapInuse":   145000000,
      "sys":         430000000,
      "goroutines":  120,
      "rssBytes":    500000000
    }
  ]
}
```

### Field Notes

- `current.memStats`: maps directly from `runtime.MemStats`, plus two derived fields:
  - `gcPauseAvgNs`: average of the last 256 GC pauses from `MemStats.PauseNs`
  - `gcPauseMaxNs`: max of the last 256 GC pauses from `MemStats.PauseNs`
- `current.goroutines.topStacks`: up to 20 entries, aggregated by stack text, sorted by count descending. Collected **only at export time** (not during background sampling). Capped at 1000 goroutine stacks to limit cost.
- `current.process`:
  - `rssBytes`: read from `/proc/self/status` VmRSS (Linux), `GetProcessMemoryInfo` WorkingSetSize (Windows), or fallback to `MemStats.Sys` (other OS)
  - `cpuPercent`: estimated from `syscall.Getrusage` or `GetProcessTimes`
  - `uptimeSeconds`: time since process start
- `history`: ring buffer of lightweight samples, 6 fields each. No goroutine stacks stored here.

## Backend: Memory Sampler

### New File: `internal/server/biz/memory_diagnostics.go`

**`MemorySampler` struct:**
- `mu sync.Mutex`
- `ring [144]MemorySample` — fixed-size ring buffer, covers 24h at 10-min intervals
- `pos int` — next write position (mod 144)
- `count int` — total samples written (up to 144 for "how full is the buffer")
- `startTime time.Time` — process start time, for uptime calculation
- `stopCh chan struct{}` — signal the sampler goroutine to stop

**`MemorySample` struct (ring buffer entry):**
- `Timestamp time.Time`
- `HeapAlloc uint64`
- `HeapInuse uint64`
- `Sys uint64`
- `Goroutines int`
- `RSSBytes uint64`

**Lifecycle (fx):**
- `OnStart`: launch `go ms.run()` which sets a `time.Ticker(10 * time.Minute)`, calls `runtime.ReadMemStats` + `runtime.NumGoroutine` + readRSS on each tick, writes to ring buffer under mutex.
- `OnStop`: close `stopCh`, wait for goroutine exit.

**`Snapshot() *MemoryDiagnosticsResult` method (called by GraphQL resolver):**
1. Call `runtime.ReadMemStats` for current memStats.
2. Collect goroutine stacks: use `runtime.NumGoroutine()` to get count, then iterate up to 1000 goroutines with `runtime.Stack` or `pprof.Lookup("goroutine")`. Aggregate by stack text, take top 20.
3. Read process RSS and CPU.
4. Read ring buffer under mutex (copy out the slice in chronological order, accounting for wrap-around).
5. Return `MemoryDiagnosticsResult` struct.

**RSS reading:**
- Platform-specific helper function `readRSS() uint64`:
  - Linux: parse `/proc/self/status` line `VmRSS:  XXXX kB`
  - Windows: `GetProcessMemoryInfo` → `PROCESS_MEMORY_COUNTERS.WorkingSetSize`
  - Other: fallback to `MemStats.Sys`

**CPU reading:**
- `readCPU() float64`: compute from process times, return percentage. On error return 0.

### GraphQL Schema Change: `internal/server/gql/system.graphql`

Add to `SystemQuery`:
```graphql
getMemoryDiagnostics: CacheDiagnosticsPayload!
```

Reuses the existing `CacheDiagnosticsPayload` type — no new GraphQL types needed.

### GraphQL Resolver: `internal/server/gql/system.resolvers.go`

Add method `GetMemoryDiagnostics`:
- Permission check: same owner-only guard as `GetCacheDiagnostics`
- Call `biz.MemorySampler.Snapshot()`
- Marshal the result to JSON
- Return `CacheDiagnosticsPayload{FileName, Content, Targets}` where:
  - `FileName` = `memory-diagnostics-YYYYMMDD-HHMMSS.json`
  - `Content` = JSON string of the result
  - `Targets` = nil (not email-specific, same as cache diagnostics)

### fx Wiring

Register `MemorySampler` as a constructor in the fx options for `internal/server/biz`. It depends on nothing external. The resolver accesses it through the existing biz client pattern (same as how cache diagnostics accesses the cache service).

## Frontend

### Data Layer: `frontend/src/features/system/data/system.ts`

Add a new query hook `useExportMemoryDiagnostics` following the exact same pattern as `useExportCacheDiagnostics`:
- GraphQL query for `getMemoryDiagnostics`
- Returns `{fileName, content, targets}`
- Blob download helper (reuse existing)

### UI: `frontend/src/features/system/components/diagnostics-settings.tsx`

Add a "Export Memory Diagnostics" button alongside the existing "Export Cache Diagnostics" button. Same styling, same loading/error handling pattern.

### i18n

Add to `frontend/src/locales/en/system.json`:
```json
"memory_diagnostics": {
  "title": "Memory Diagnostics",
  "export": "Export Memory Diagnostics",
  "description": "Export current memory usage, goroutine summary, and 24h trend data for leak analysis.",
  "exporting": "Exporting memory diagnostics...",
  "success": "Memory diagnostics exported successfully",
  "error": "Failed to export memory diagnostics"
}
```

Add to `frontend/src/locales/zh-CN/system.json`:
```json
"memory_diagnostics": {
  "title": "内存诊断",
  "export": "导出内存诊断",
  "description": "导出当前内存使用情况、Goroutine 摘要及 24 小时趋势数据，用于排查内存泄漏。",
  "exporting": "正在导出内存诊断...",
  "success": "内存诊断导出成功",
  "error": "内存诊断导出失败"
}
```

## What We Are NOT Doing

- **No real-time dashboard/SSE.** The use case is "download a file to share or compare," not "watch a live chart."
- **No goroutine stack sampling in the background.** Too expensive for a 10-min ticker. Only collected at export time.
- **No configurable sampling interval.** Fixed at 10 minutes. YAGNI.
- **No ring buffer persistence.** Data is lost on process restart. For leak analysis, 24h of data is sufficient.
- **No pprof binary format.** User explicitly chose readable JSON over .pprof files.

## Security

- Owner-only access, same as cache diagnostics and backup.
- No new authentication or authorization mechanisms — reuse existing GraphQL guard.
- Exported data does not contain request bodies, API keys, or user data — only runtime statistics and stack traces of internal goroutines.
