//go:build linux

package biz

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func collectSystemRuntimeMetrics() systemRuntimeRawMetrics {
	idle, total := linuxSystemCPUTimes()
	memoryUsed, memoryTotal := linuxMemoryUsage()
	networkReceived, networkTransmitted := linuxNetworkCounters()
	diskUsed, diskTotal := linuxDiskUsage()
	load1, load5, load15 := linuxLoadAverage()
	return systemRuntimeRawMetrics{
		SystemCPUIdle:      idle,
		SystemCPUTotal:     total,
		ProcessCPUNanos:    linuxProcessCPUNanos(),
		MemoryUsedBytes:    memoryUsed,
		MemoryTotalBytes:   memoryTotal,
		NetworkReceived:    networkReceived,
		NetworkTransmitted: networkTransmitted,
		DiskUsedBytes:      diskUsed,
		DiskTotalBytes:     diskTotal,
		Load1:              load1,
		Load5:              load5,
		Load15:             load15,
		ProcessRSSBytes:    readRSSPlatform(),
		ProcessThreads:     linuxProcessThreads(),
	}
}

func collectSystemRuntimeHostDetails() systemRuntimeHostDetails {
	platform, version := linuxOSRelease()
	kernelBytes, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return systemRuntimeHostDetails{
		Platform:        platform,
		PlatformVersion: version,
		KernelVersion:   strings.TrimSpace(string(kernelBytes)),
	}
}

func linuxSystemCPUTimes() (uint64, uint64) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, 0
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, 0
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return idle, total
}

func linuxProcessCPUNanos() uint64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	return timevalNanos(usage.Utime) + timevalNanos(usage.Stime)
}

func timevalNanos(value syscall.Timeval) uint64 {
	return uint64(value.Sec)*1_000_000_000 + uint64(value.Usec)*1_000
}

func linuxMemoryUsage() (uint64, uint64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var total, available uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			total = value * 1024
		case "MemAvailable":
			available = value * 1024
		}
	}
	if total < available {
		return 0, 0
	}
	return total - available, total
}

func linuxNetworkCounters() (uint64, uint64) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	var received, transmitted uint64
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		rx, rxErr := strconv.ParseUint(fields[0], 10, 64)
		tx, txErr := strconv.ParseUint(fields[8], 10, 64)
		if rxErr == nil {
			received += rx
		}
		if txErr == nil {
			transmitted += tx
		}
	}
	return received, transmitted
}

func linuxDiskUsage() (uint64, uint64) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return 0, 0
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(workingDirectory, &stat); err != nil {
		return 0, 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total < free {
		return 0, 0
	}
	return total - free, total
}

func linuxLoadAverage() (float64, float64, float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	load1, _ := strconv.ParseFloat(fields[0], 64)
	load5, _ := strconv.ParseFloat(fields[1], 64)
	load15, _ := strconv.ParseFloat(fields[2], 64)
	return load1, load5, load15
}

func linuxProcessThreads() int {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "Threads:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 2 {
			threads, _ := strconv.Atoi(fields[1])
			return threads
		}
	}
	return 0
}

func linuxOSRelease() (string, string) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux", ""
	}
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			values[parts[0]] = strings.Trim(parts[1], "\"")
		}
	}
	platform := values["NAME"]
	if platform == "" {
		platform = "Linux"
	}
	return platform, values["VERSION_ID"]
}
