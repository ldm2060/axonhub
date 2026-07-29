//go:build windows

package biz

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32Runtime              = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimesRuntime       = modKernel32Runtime.NewProc("GetSystemTimes")
	procGlobalMemoryStatusExRuntime = modKernel32Runtime.NewProc("GlobalMemoryStatusEx")
)

type runtimeMemoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func collectSystemRuntimeMetrics() systemRuntimeRawMetrics {
	idle, total := windowsSystemCPUTimes()
	processCPU := windowsProcessCPUNanos()
	memoryUsed, memoryTotal := windowsMemoryUsage()
	networkReceived, networkTransmitted := windowsNetworkCounters()
	diskUsed, diskTotal := windowsDiskUsage()
	return systemRuntimeRawMetrics{
		SystemCPUIdle:      idle,
		SystemCPUTotal:     total,
		ProcessCPUNanos:    processCPU,
		MemoryUsedBytes:    memoryUsed,
		MemoryTotalBytes:   memoryTotal,
		NetworkReceived:    networkReceived,
		NetworkTransmitted: networkTransmitted,
		DiskUsedBytes:      diskUsed,
		DiskTotalBytes:     diskTotal,
		ProcessRSSBytes:    readRSSPlatform(),
	}
}

func collectSystemRuntimeHostDetails() systemRuntimeHostDetails {
	version := windows.RtlGetVersion()
	platformVersion := fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.BuildNumber)
	return systemRuntimeHostDetails{
		Platform:        "Windows",
		PlatformVersion: platformVersion,
		KernelVersion:   platformVersion,
	}
}

func windowsSystemCPUTimes() (uint64, uint64) {
	var idleTime, kernelTime, userTime windows.Filetime
	ret, _, _ := procGetSystemTimesRuntime.Call(
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if ret == 0 {
		return 0, 0
	}
	idle := filetimeValue(idleTime)
	kernel := filetimeValue(kernelTime)
	user := filetimeValue(userTime)
	return idle, kernel + user
}

func windowsProcessCPUNanos() uint64 {
	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creationTime, &exitTime, &kernelTime, &userTime); err != nil {
		return 0
	}
	return (filetimeValue(kernelTime) + filetimeValue(userTime)) * 100
}

func windowsMemoryUsage() (uint64, uint64) {
	status := runtimeMemoryStatusEx{Length: uint32(unsafe.Sizeof(runtimeMemoryStatusEx{}))}
	ret, _, _ := procGlobalMemoryStatusExRuntime.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 || status.TotalPhys < status.AvailPhys {
		return 0, 0
	}
	return status.TotalPhys - status.AvailPhys, status.TotalPhys
}

func windowsNetworkCounters() (uint64, uint64) {
	var table *windows.MibIfTable2
	if err := windows.GetIfTable2Ex(windows.MibIfTableNormal, &table); err != nil || table == nil {
		return 0, 0
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))

	rows := unsafe.Slice(&table.Table[0], int(table.NumEntries))
	var received, transmitted uint64
	for _, row := range rows {
		if row.Type == windows.IF_TYPE_SOFTWARE_LOOPBACK || row.OperStatus != windows.IfOperStatusUp {
			continue
		}
		received += row.InOctets
		transmitted += row.OutOctets
	}
	return received, transmitted
}

func windowsDiskUsage() (uint64, uint64) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return 0, 0
	}
	pathPtr, err := windows.UTF16PtrFromString(workingDirectory)
	if err != nil {
		return 0, 0
	}
	var freeAvailable, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeAvailable, &total, &free); err != nil || total < free {
		return 0, 0
	}
	return total - free, total
}

func filetimeValue(value windows.Filetime) uint64 {
	return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
}

var _ = runtime.GOOS
