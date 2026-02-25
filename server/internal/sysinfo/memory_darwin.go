//go:build darwin

package sysinfo

import (
	"log"
	"syscall"
	"unsafe"
)

// TotalMemoryBytes returns the total physical memory of the host in bytes.
// On macOS, it uses sysctl HW_MEMSIZE. Falls back to 8GB if unavailable.
func TotalMemoryBytes() uint64 {
	mib := []int32{6 /* CTL_HW */, 24 /* HW_MEMSIZE */}
	var memSize uint64

	n := uintptr(8) // size of uint64
	_, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(len(mib)),
		uintptr(unsafe.Pointer(&memSize)),
		uintptr(unsafe.Pointer(&n)),
		0,
		0,
	)

	if errno != 0 {
		log.Printf("Failed to get system memory via sysctl, using 8GB default: %v", errno)
		return fallbackMemory
	}

	return memSize
}
