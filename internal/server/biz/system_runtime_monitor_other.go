//go:build !linux && !windows

package biz

import "runtime"

func collectSystemRuntimeMetrics() systemRuntimeRawMetrics {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return systemRuntimeRawMetrics{
		MemoryUsedBytes:  mem.Sys,
		MemoryTotalBytes: mem.Sys,
		ProcessRSSBytes:  readRSSPlatform(),
	}
}

func collectSystemRuntimeHostDetails() systemRuntimeHostDetails {
	return systemRuntimeHostDetails{Platform: runtime.GOOS}
}
