//go:build windows

package sysinfo

import (
	"log"
	"syscall"
	"unsafe"
)

type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

var (
	modkernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modkernel32.NewProc("GlobalMemoryStatusEx")
)

// TotalMemoryBytes returns the total physical memory of the host in bytes.
// On Windows, it uses GlobalMemoryStatusEx. Falls back to 8GB if unavailable.
func TotalMemoryBytes() uint64 {
	var memStatus memoryStatusEx
	memStatus.length = uint32(unsafe.Sizeof(memStatus))

	ret, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret == 0 {
		log.Printf("Failed to get system memory via GlobalMemoryStatusEx, using 8GB default: %v", err)
		return fallbackMemory
	}

	return memStatus.totalPhys
}
