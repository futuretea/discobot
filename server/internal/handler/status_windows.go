//go:build windows

package handler

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// getDiskUsage returns filesystem usage statistics for a given path
func getDiskUsage(path string) *DiskUsageInfo {
	// Convert to UTF16 for Windows API
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil
	}

	var freeBytesAvailable uint64
	var totalBytes uint64
	var totalFreeBytes uint64

	err = windows.GetDiskFreeSpaceEx(
		pathPtr,
		&freeBytesAvailable,
		&totalBytes,
		&totalFreeBytes,
	)
	if err != nil {
		return nil
	}

	usedBytes := totalBytes - totalFreeBytes

	var usedPercent float64
	if totalBytes > 0 {
		usedPercent = float64(usedBytes) / float64(totalBytes) * 100
	}

	return &DiskUsageInfo{
		TotalBytes:     totalBytes,
		UsedBytes:      usedBytes,
		AvailableBytes: freeBytesAvailable,
		UsedPercent:    usedPercent,
	}
}

// getDataDiskFiles scans for project data disk images and returns their size info.
// On Windows, we report both apparent and actual size as the file size.
// Sparse file detection on Windows requires more complex Win32 API calls.
func getDataDiskFiles(dataDir string) []DataDiskFileInfo {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil
	}

	var disks []DataDiskFileInfo
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "project-") || !strings.HasSuffix(name, "-data.img") {
			continue
		}

		path := filepath.Join(dataDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		apparentBytes := uint64(info.Size())
		// On Windows, report actual size same as apparent size
		// Full sparse file support would require DeviceIoControl with FSCTL_GET_COMPRESSION
		actualBytes := apparentBytes

		disks = append(disks, DataDiskFileInfo{
			Path:          path,
			ApparentBytes: apparentBytes,
			ActualBytes:   actualBytes,
		})
	}

	return disks
}
