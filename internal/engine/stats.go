package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pb "gobackup/internal/grpc/pb"
)

func GetStorageStats(cfg Config) ([]*pb.StorageStat, int64, int64, int64, int32, int32, int32) {
	var stats []*pb.StorageStat

	addedPaths := make(map[string]bool)
	dirs := []struct{ Path, Tier string }{
		{cfg.BackupDir, "HOT"},
	}
	addedPaths[cfg.BackupDir] = true

	if cfg.ColdStoragePath != "" {
		dirs = append(dirs, struct{ Path, Tier string }{cfg.ColdStoragePath, "COLD"})
		addedPaths[cfg.ColdStoragePath] = true
	}

	for _, job := range cfg.Jobs {
		if job.HotStoragePath != "" && !addedPaths[job.HotStoragePath] {
			dirs = append(dirs, struct{ Path, Tier string }{job.HotStoragePath, "HOT"})
			addedPaths[job.HotStoragePath] = true
		}

		coldPathStr := job.ColdStorage.Path
		if coldPathStr == "" {
			coldPathStr = cfg.ColdStoragePath
		}
		if coldPathStr != "" && !addedPaths[coldPathStr] {
			dirs = append(dirs, struct{ Path, Tier string }{coldPathStr, "COLD"})
			addedPaths[coldPathStr] = true
		}
	}

	var globalTotalDisk, globalFreeDisk, globalUsedDisk int64
	var globalTotalBackups int32
	var globalHotBackups int32
	var globalColdBackups int32

	for _, d := range dirs {
		dTotal, dFree, dUsed := getDiskInfo(d.Path)
		var dBackups int32 = 0
		if files, err := os.ReadDir(d.Path); err == nil {
			for _, entry := range files {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tar.gz") {
					dBackups++
				}
			}
		}
		// Also scan docker-volume subdirectory
		if files, err := os.ReadDir(filepath.Join(d.Path, "docker-volume")); err == nil {
			for _, entry := range files {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tar.gz") {
					dBackups++
				}
			}
		}

		stats = append(stats, &pb.StorageStat{
			Path:         d.Path,
			Tier:         d.Tier,
			DiskTotal:    dTotal,
			DiskFree:     dFree,
			DiskUsed:     dUsed,
			TotalBackups: dBackups,
		})

		// Only aggregate global stats for HOT drives to prevent double-counting massive NAS drives
		if d.Tier == "HOT" {
			globalTotalDisk += dTotal
			globalFreeDisk += dFree
			globalUsedDisk += dUsed
			globalHotBackups += dBackups
		} else if d.Tier == "COLD" {
			globalColdBackups += dBackups
		}
		globalTotalBackups += dBackups
	}

	return stats, globalTotalDisk, globalFreeDisk, globalUsedDisk, globalTotalBackups, globalHotBackups, globalColdBackups
}

func FormatStorageStatsMarkdown(stats []*pb.StorageStat) string {
	var sb strings.Builder
	sb.WriteString("\n\n**Storage Telemetry**\n")
	sb.WriteString("\x60\x60\x60text\n")
	sb.WriteString(fmt.Sprintf("%-6s | %-20s | %-8s | %-10s | %-10s | %-10s\n", "TIER", "PATH", "ARCHIVES", "USED", "TOTAL", "FREE"))
	sb.WriteString("--------------------------------------------------------------------------------------\n")

	for _, s := range stats {
		pathStr := s.Path
		if len(pathStr) > 20 { pathStr = "..." + pathStr[len(pathStr)-17:] }
		
		sb.WriteString(fmt.Sprintf("%-6s | %-20s | %-8d | %-10s | %-10s | %-10s\n",
			s.Tier, pathStr, s.TotalBackups, formatSize(s.DiskUsed), formatSize(s.DiskTotal), formatSize(s.DiskFree)))
	}
	sb.WriteString("\x60\x60\x60\n")
	return sb.String()
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
