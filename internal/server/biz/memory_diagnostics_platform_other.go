//go:build !linux && !windows

package biz

import "runtime"

func readRSSPlatform() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys
}

func readCPUPlatform() float64 {
	return 0
}
