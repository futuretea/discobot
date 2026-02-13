//go:build unix

package handler

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// getDiskUsage returns filesystem usage statistics for a given path
func getDiskUsage(path string) *DiskUsageInfo {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - (stat.Bfree * uint64(stat.Bsize))

	var usedPercent float64
	if totalBytes > 0 {
		usedPercent = float64(usedBytes) / float64(totalBytes) * 100
	}

	return &DiskUsageInfo{
		TotalBytes:     totalBytes,
		UsedBytes:      usedBytes,
		AvailableBytes: availableBytes,
		UsedPercent:    usedPercent,
	}
}

// getDataDiskFiles scans for project data disk images and returns their size info.
// Data disks are sparse files, so actual disk usage may be much less than apparent size.
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

		// Get actual disk usage via stat blocks (sparse-aware)
		var stat syscall.Stat_t
		var actualBytes uint64
		if err := syscall.Stat(path, &stat); err == nil {
			actualBytes = uint64(stat.Blocks) * 512
		}

		disks = append(disks, DataDiskFileInfo{
			Path:          path,
			ApparentBytes: apparentBytes,
			ActualBytes:   actualBytes,
		})
	}

	return disks
}
