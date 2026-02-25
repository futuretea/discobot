//go:build linux

package sysinfo

import (
	"log"
	"syscall"
)

// TotalMemoryBytes returns the total physical memory of the host in bytes.
// On Linux, it uses the Sysinfo syscall. Falls back to 8GB if unavailable.
func TotalMemoryBytes() uint64 {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		log.Printf("Failed to get system memory via sysinfo, using 8GB default: %v", err)
		return fallbackMemory
	}

	return info.Totalram
}
