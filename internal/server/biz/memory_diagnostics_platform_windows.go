//go:build windows

package biz

import (
	"syscall"
	"unsafe"
)

var (
	modpsapi                 = syscall.NewLazyDLL("psapi.dll")
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
	// GetCurrentProcess returns a pseudo-handle with value -1
	currentProcess := ^uintptr(0) // -1 = GetCurrentProcess pseudo-handle
	var counters PROCESS_MEMORY_COUNTERS
	counters.CB = uint32(unsafe.Sizeof(counters))

	ret, _, _ := procGetProcessMemoryInfo.Call(
		currentProcess,
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
