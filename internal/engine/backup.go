package engine

import (
	"sync"
	"context"
	"fmt"
	
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	
	"time"

	"gobackup/internal/tui"
)

type cmdLogger struct {
	ui tui.BackupUI
}

func (c *cmdLogger) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			c.ui.Log("%s", line)
		}
	}
	return len(p), nil
}





var GlobalBackupQueue sync.Mutex

var (
	StateMutex sync.Mutex
	ActiveJob  string
	QueuedJobs []string
)

func verifyNFSMount(mountPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		sentinelPath := filepath.Join(mountPath, ".gobackup_mounted")
		_, err := os.Stat(sentinelPath)
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("NFS Mount is stale or unresponsive (timeout after 3s)")
	case err := <-errCh:
		if os.IsNotExist(err) {
			return fmt.Errorf("Target storage volume is unmounted (missing .gobackup_mounted)")
		}
		if err != nil {
			return fmt.Errorf("NFS check failed: %v", err)
		}
		return nil
	}
}

type JobResult struct {
	Server   string
	Job      string
	Status   string // "SUCCESS", "WARNING", "FAILED"
	Error    string
	Duration string
}

var (
	IsBatchActive bool
	BatchResults  []JobResult
	BatchStartTime time.Time
	BatchEnqueuedJobs []string
)

func EnqueueJob(host string, cfg Config) {
	StateMutex.Lock()
	
	// Prevent duplicates so we can pre-populate global runs safely
	isDup := false
	for _, v := range QueuedJobs {
		if v == host {
			isDup = true
			break
		}
	}
	if !isDup {
		QueuedJobs = append(QueuedJobs, host)
		if IsBatchActive {
			BatchEnqueuedJobs = append(BatchEnqueuedJobs, host)
		}
	}

	if !IsBatchActive && len(QueuedJobs) > 0 {
		IsBatchActive = true
		BatchResults = nil
		BatchEnqueuedJobs = []string{host}
		BatchStartTime = time.Now()

		// Launch a debouncer that waits 2 seconds for all cron jobs to enter the queue, then sends the batch start alert
		go func(c Config) {
			time.Sleep(2 * time.Second)
			StateMutex.Lock()
			
			// If the batch completely finished before the 2-second debouncer woke up (a hyper-fast job), abort!
			if !IsBatchActive {
				StateMutex.Unlock()
				return
			}
			
			jobsList := make([]string, len(BatchEnqueuedJobs))
			copy(jobsList, BatchEnqueuedJobs)
			
			// Highlight the active job
			for i, j := range jobsList {
				if j == ActiveJob {
					jobsList[i] = j + " (Running)"
				}
			}
			StateMutex.Unlock()

			desc := "**The following jobs have been queued for execution:**\n"
			for _, j := range jobsList {
				display := strings.Replace(j, "_", "/", 1)
				if strings.Contains(j, "(Running)") {
					desc += "🔥 **" + display + "**\n"
				} else {
					desc += "⏳ **" + display + "**\n"
				}
			}
			
			SendNotification(c.Notifications, "🚀 Backup Queue Started", desc, 0x3498DB, "Multiple Targets", fmt.Sprintf("%d jobs in queue", len(jobsList)), nil)
		}(cfg)
	}
	
	StateMutex.Unlock()
}

func DequeueAndSetActive(host string) {
	StateMutex.Lock()
	defer StateMutex.Unlock()
	for i, v := range QueuedJobs {
		if v == host {
			QueuedJobs = append(QueuedJobs[:i], QueuedJobs[i+1:]...)
			break
		}
	}
	ActiveJob = host
}

func ClearActive(cfg Config) {
	StateMutex.Lock()
	defer StateMutex.Unlock()
	ActiveJob = ""

	if IsBatchActive && len(QueuedJobs) == 0 {
		IsBatchActive = false
		
		// Build the digest!
		successCount := 0
		warnCount := 0
		failCount := 0
		
		desc := "**Batch Execution Summary:**\n"
		
		color := 0x00FF00 // Default to green
		
		for _, res := range BatchResults {
			if res.Status == "SUCCESS" {
				successCount++
				desc += fmt.Sprintf("✅ **%s/%s** - %s\n", res.Server, res.Job, res.Duration)
			} else if res.Status == "WARNING" {
				warnCount++
				color = 0xF1C40F // Yellow
				desc += fmt.Sprintf("⚠️ **%s/%s** - %s\n", res.Server, res.Job, res.Error)
			} else {
				failCount++
				color = 0xFF0000 // Red
				desc += fmt.Sprintf("❌ **%s/%s** - %s\n", res.Server, res.Job, res.Error)
			}
		}
		
		title := fmt.Sprintf("✅ Backup Digest: %d Succ, %d Warn, %d Fail", successCount, warnCount, failCount)
		if failCount > 0 {
			title = fmt.Sprintf("❌ Backup Digest: %d Succ, %d Warn, %d Fail", successCount, warnCount, failCount)
		} else if warnCount > 0 {
			title = fmt.Sprintf("⚠️ Backup Digest: %d Succ, %d Warn, %d Fail", successCount, warnCount, failCount)
		}

		SendNotification(cfg.Notifications, title, desc, color, "Batch Digest", fmt.Sprintf("Total Duration: %s", time.Since(BatchStartTime).Round(time.Second).String()), nil)
	}
}

func RunBackups(cfg Config, ui tui.BackupUI) {
	for _, job := range cfg.Jobs {
		EnqueueJob(job.Server + "_" + job.Name, cfg)
		go RunSingleBackup(cfg, job, ui)
	}
	CleanupOldBackups(cfg.BackupDir, cfg.Jobs, ui)
}


func isTimeForFullBackup(backupDir string, job JobConfig, volName string) bool {
	// Look for the newest FULL backup in backupDir
	entries, err := os.ReadDir(backupDir)
	if err != nil { return true }
	
	prefix := fmt.Sprintf("%s_%s_", job.Server, job.Name)
	if volName != "" {
		prefix += volName + "_"
	}
	prefix += "FULL_"
	
	var newestFull *os.FileInfo
	
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			info, err := entry.Info()
			if err == nil {
				if newestFull == nil || info.ModTime().After((*newestFull).ModTime()) {
					newestFull = &info
				}
			}
		}
	}
	
	if newestFull == nil { return true } // No full backup exists
	
	age := time.Since((*newestFull).ModTime())
	if job.FullInterval <= 0 { job.FullInterval = 7 }
	if age.Hours() > float64(job.FullInterval * 24) {
		return true // Older than interval
	}
	return false
}

func resetSnapshot(srv ServerConfig, job JobConfig, volName string, ui tui.BackupUI) {
	snarFile := fmt.Sprintf("/home/backup/.gobackup/snapshots/%s_%s.snar", job.Server, job.Name)
	if volName != "" {
		snarFile = fmt.Sprintf("/home/backup/.gobackup/snapshots/%s_%s_%s.snar", job.Server, job.Name, volName)
	}
	
	ui.Log("   ♻️ Resetting incremental chain (deleting %s)...", snarFile)
	
	var cmd *exec.Cmd
	resetCmd := fmt.Sprintf("mkdir -p /home/backup/.gobackup/snapshots && rm -f %s", snarFile)
	
	if srv.Address == "localhost" || srv.Address == "127.0.0.1" || srv.Address == "local" {
		cmd = exec.Command("sh", "-c", resetCmd)
	} else {
		args := []string{"-p", strconv.Itoa(srv.Port), srv.Address, resetCmd}
		cmd = exec.Command("ssh", args...)
	}
	cmd.Run()
}

func RunSingleBackup(cfg Config, job JobConfig, ui tui.BackupUI) {
	if err := verifyNFSMount(cfg.BackupDir); err != nil {
		ui.Log("❌ Backup Failed: %v", err)
		SendNotification(cfg.Notifications, "❌ Critical Storage Error", fmt.Sprintf("Backup aborted for **%s/%s**.\n\n%v", job.Server, job.Name, err), 0xFF0000, job.Server, cfg.BackupDir, ui)
		return
	}
	var targetServer *ServerConfig
	for _, srv := range cfg.Servers {
		if srv.Name == job.Server {
			targetServer = &srv
			break
		}
	}
	if targetServer == nil {
		ui.Log("❌ Failed to find server for job %s", job.Name)
		return
	}
	srv := *targetServer

	ui.SetStatus(fmt.Sprintf("Queued: %s", job.Name), true)
	ui.Log("⏳ Job for %s entered the global queue. Waiting for active jobs to finish...", job.Name)
	
	EnqueueJob(job.Server + "_" + job.Name, cfg)

	GlobalBackupQueue.Lock()
	DequeueAndSetActive(job.Server + "_" + job.Name)

	defer func() {
		ClearActive(cfg)
		GlobalBackupQueue.Unlock()
	}()

	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		ui.Log("❌ Failed to create backup directory %s: %v", cfg.BackupDir, err)
		return
	}
	
	dockerDir := filepath.Join(cfg.BackupDir, "docker-volume")
	if len(job.DockerVolumes) > 0 {
		if err := os.MkdirAll(dockerDir, 0755); err != nil {
			ui.Log("❌ Failed to create docker-volume directory %s: %v", dockerDir, err)
			return
		}
	}

	if srv.Port == 0 {
		srv.Port = 22
	}

	// 1. System Backups
	if len(job.Paths) > 0 {
		isFull := true
		if job.Incremental {
			isFull = isTimeForFullBackup(cfg.BackupDir, job, "")
		}
		archiveType := "FULL"
		if job.Incremental && !isFull { archiveType = "INC" }
		if job.Incremental && isFull { resetSnapshot(srv, job, "", ui) }
		
		timestamp := time.Now().Format("20060102_150405")
		fileName := fmt.Sprintf("%s_%s_%s_%s.tar.gz", job.Server, job.Name, archiveType, timestamp)
		targetFile := filepath.Join(cfg.BackupDir, fileName)

		// Pre-create snapshots dir just in case it doesn't exist
		if job.Incremental {
			mkdirCmdStr := "mkdir -p /home/backup/.gobackup/snapshots"
			var preCmd *exec.Cmd
			if srv.Address == "localhost" || srv.Address == "127.0.0.1" || srv.Address == "local" {
				preCmd = exec.Command("sh", "-c", mkdirCmdStr)
			} else {
				preArgs := []string{"-p", strconv.Itoa(srv.Port), srv.Address, mkdirCmdStr}
				preCmd = exec.Command("ssh", preArgs...)
			}
			preCmd.Run()
		}

		var cmd *exec.Cmd
		tarArgs := []string{"-cvzf", "-"}
		if job.Incremental {
			tarArgs = append(tarArgs, "-g", fmt.Sprintf("/home/backup/.gobackup/snapshots/%s_%s.snar", job.Server, job.Name))
		}
		tarArgs = append(tarArgs, job.Paths...)

		if srv.Address == "localhost" || srv.Address == "127.0.0.1" || srv.Address == "local" {
			var args []string
			if srv.UseSudo {
				args = append([]string{"tar"}, tarArgs...)
				cmd = exec.Command("sudo", args...)
			} else {
				cmd = exec.Command("tar", tarArgs...)
			}
		} else {
			args := []string{"-p", strconv.Itoa(srv.Port), srv.Address}
			if srv.UseSudo {
				args = append(args, "sudo", "-n", "tar")
			} else {
				args = append(args, "tar")
			}
			args = append(args, tarArgs...)
			cmd = exec.Command("ssh", args...)
		}
		
		executeBackupCommand(cfg, job, srv, ui, cmd, targetFile, "SYSTEM")
	}

	// 2. Docker Backups
	for _, vol := range job.DockerVolumes {
		isFull := true
		if job.Incremental {
			isFull = isTimeForFullBackup(dockerDir, job, vol)
		}
		archiveType := "FULL"
		if job.Incremental && !isFull { archiveType = "INC" }
		if job.Incremental && isFull { resetSnapshot(srv, job, vol, ui) }
		
		timestamp := time.Now().Format("20060102_150405")
		fileName := fmt.Sprintf("%s_%s_%s_%s_%s.tar.gz", job.Server, job.Name, vol, archiveType, timestamp)
		targetFile := filepath.Join(dockerDir, fileName)

		// Pre-create snapshots dir to prevent Docker from creating it as root:root
		if job.Incremental {
			mkdirCmdStr := "mkdir -p /home/backup/.gobackup/snapshots"
			var preCmd *exec.Cmd
			if srv.Address == "localhost" || srv.Address == "127.0.0.1" || srv.Address == "local" {
				preCmd = exec.Command("sh", "-c", mkdirCmdStr)
			} else {
				preArgs := []string{"-p", strconv.Itoa(srv.Port), srv.Address, mkdirCmdStr}
				preCmd = exec.Command("ssh", preArgs...)
			}
			preCmd.Run()
		}

		var cmd *exec.Cmd
		
		dockerCmdStr := fmt.Sprintf("docker run --rm -v %s:/volume:ro ", vol)
		if job.Incremental {
			dockerCmdStr += fmt.Sprintf("-v /home/backup/.gobackup/snapshots:/snapshots debian:stable-slim tar -cvzf - -C /volume -g /snapshots/%s_%s_%s.snar .", job.Server, job.Name, vol)
		} else {
			dockerCmdStr += "debian:stable-slim tar -cvzf - -C /volume ."
		}
		
		if srv.UseSudo {
			dockerCmdStr = "sudo -n " + dockerCmdStr
		}
		
		if srv.Address == "localhost" || srv.Address == "127.0.0.1" || srv.Address == "local" {
			cmd = exec.Command("sh", "-c", dockerCmdStr)
		} else {
			args := []string{"-p", strconv.Itoa(srv.Port), srv.Address, dockerCmdStr}
			cmd = exec.Command("ssh", args...)
		}

		executeBackupCommand(cfg, job, srv, ui, cmd, targetFile, fmt.Sprintf("DOCKER VOLUME (%s)", vol))
	}
	
	// Prune just this host after it finishes
	CleanupOldBackups(cfg.BackupDir, []JobConfig{job}, ui)
}

func executeBackupCommand(cfg Config, job JobConfig, srv ServerConfig, ui tui.BackupUI, cmd *exec.Cmd, targetFile string, backupType string) {
	outFile, err := os.Create(targetFile)
	if err != nil {
		ui.Log("   ❌ Error creating local file: %v", err)
		return
	}
	cmd.Stdout = outFile
	cmd.Stderr = &cmdLogger{ui: ui} 

	
	startTime := time.Now()

	ui.SetStatus(fmt.Sprintf("Backing up %s for: %s/%s (%s)", backupType, job.Server, job.Name, srv.Address), true)

	err = cmd.Run()
	duration := time.Since(startTime).Round(time.Second)
	outFile.Close()

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1 // Unknown error (like command not found)
		}
	}

	if exitCode != 0 && exitCode != 1 {
		pushJobResult(job.Server, job.Name, "FAILED", fmt.Sprintf("Exit Code: %d", exitCode), duration.String())

		ui.Summary("   ❌ %s Backup fatally failed for %s/%s (Exit Code %d): %v", backupType, job.Server, job.Name, exitCode, err)
		os.Remove(targetFile) 
		return
	}
	
	var sizeStr string
	if info, e := os.Stat(targetFile); e == nil {
		sizeStr = fmt.Sprintf("%.2f MB", float64(info.Size())/1024.0/1024.0)
	}

	if exitCode == 1 {
		pushJobResult(job.Server, job.Name, "WARNING", "Files changed during run", duration.String())
		ui.Summary("   ⚠️  %s Completed with warnings (files changed) for %s/%s (%s)", backupType, job.Server, job.Name, sizeStr)
	} else {
		pushJobResult(job.Server, job.Name, "SUCCESS", "", duration.String())
		ui.Summary("   ✅ Success (%s)! Saved to %s (%s) in %s", backupType, targetFile, sizeStr, duration)
	}
}

func moveFileAcrossPartitions(src, dst string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	
	out, err := os.Create(dst)
	if err != nil {
		in.Close()
		return err
	}
	
	_, err = io.Copy(out, in)
	in.Close()
	out.Close()
	
	if err != nil { return err }
	return os.Remove(src)
}

func CleanupOldBackups(dir string, jobs []JobConfig, ui tui.BackupUI) {
	if err := verifyNFSMount(dir); err != nil {
		ui.Log("❌ Prune Failed: %v", err)
		return
	}
	dirsToClean := []string{dir, filepath.Join(dir, "docker-volume")}

	for _, cleanDir := range dirsToClean {
		files, err := os.ReadDir(cleanDir)
		if err != nil {
			continue // Skip if dir doesn't exist
		}

		for _, job := range jobs {
			if job.RetentionCount <= 0 {
				continue
			}
			
			// We track chains per volume (or "" for system)
			vols := job.DockerVolumes
			if len(vols) == 0 { vols = []string{""} } else { vols = append(vols, "") } // job might have both system and docker
			
			for _, vol := range vols {
				var hostBackups []os.FileInfo
				for _, entry := range files {
					if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
						continue
					}
					prefix := job.Server + "_" + job.Name + "_"
					if vol != "" { prefix += vol + "_" }
					
					// To ensure we don't mix system paths with docker volumes when they share prefixes
					if strings.HasPrefix(entry.Name(), prefix) {
						// For system backups (vol == ""), make sure it doesn't accidentally match a docker volume
						// e.g. server_job_vol_FULL vs server_job_FULL
						// We can just check the number of parts
						parts := strings.Split(entry.Name(), "_")
						isDockerFile := len(parts) >= 6
						
						if vol == "" && isDockerFile { continue }
						if vol != "" && !isDockerFile { continue }
						
						info, err := entry.Info()
						if err == nil {
							hostBackups = append(hostBackups, info)
						}
					}
				}
	
				sort.Slice(hostBackups, func(i, j int) bool {
					return hostBackups[i].ModTime().Before(hostBackups[j].ModTime()) // Oldest first
				})
				
				var chains [][]os.FileInfo
				for _, backup := range hostBackups {
					isInc := strings.Contains(backup.Name(), "_INC_")
					
					if !isInc || len(chains) == 0 {
						// Start a new chain
						chains = append(chains, []os.FileInfo{backup})
					} else {
						// Append to latest chain
						chains[len(chains)-1] = append(chains[len(chains)-1], backup)
					}
				}
				
				// Keep RetentionCount chains
				if len(chains) > job.RetentionCount {
					numToDelete := len(chains) - job.RetentionCount
					for i := 0; i < numToDelete; i++ {
						for _, oldBackup := range chains[i] {
							oldPath := filepath.Join(cleanDir, oldBackup.Name())
							
							// Check if Cold Storage is configured
							if job.ColdStorage.Path != "" {
								coldDir := job.ColdStorage.Path
								// If it's a docker volume backup, it goes into the docker-volume subfolder
								if strings.Contains(cleanDir, "docker-volume") {
									coldDir = filepath.Join(job.ColdStorage.Path, "docker-volume")
								}
								os.MkdirAll(coldDir, 0755)
								newPath := filepath.Join(coldDir, oldBackup.Name())
								
								// Move file instead of deleting
								err := moveFileAcrossPartitions(oldPath, newPath)
								if err != nil {
									ui.Summary("   ❌ Failed to move %s to cold storage: %v", oldBackup.Name(), err)
								} else {
									ui.Summary("   ❄️  Moved expiring backup to cold storage: %s", oldBackup.Name())
								}
							} else {
								os.Remove(oldPath)
								ui.Summary("   🗑️  Pruned old backup for %s: %s", job.Name, oldBackup.Name())
							}
						}
					}
				}
			}
		}
	}
	
	CleanupColdStorage(jobs, ui)
}

func CleanupColdStorage(jobs []JobConfig, ui tui.BackupUI) {
	for _, job := range jobs {
		if job.ColdStorage.Path == "" || job.ColdStorage.RetentionCount <= 0 { continue }
		if err := verifyNFSMount(job.ColdStorage.Path); err != nil {
			ui.Log("❌ Cold Prune Failed for %s: %v", job.Name, err)
			continue
		}
		
		dirsToClean := []string{job.ColdStorage.Path, filepath.Join(job.ColdStorage.Path, "docker-volume")}
		for _, cleanDir := range dirsToClean {
			files, err := os.ReadDir(cleanDir)
			if err != nil { continue }
			
			vols := job.DockerVolumes
			if len(vols) == 0 { vols = []string{""} } else { vols = append(vols, "") }
			
			for _, vol := range vols {
				var hostBackups []os.FileInfo
				for _, entry := range files {
					if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") { continue }
					prefix := job.Server + "_" + job.Name + "_"
					if vol != "" { prefix += vol + "_" }
					
					if strings.HasPrefix(entry.Name(), prefix) {
						parts := strings.Split(entry.Name(), "_")
						isDockerFile := len(parts) >= 6
						if vol == "" && isDockerFile { continue }
						if vol != "" && !isDockerFile { continue }
						
						info, err := entry.Info()
						if err == nil { hostBackups = append(hostBackups, info) }
					}
				}
				
				sort.Slice(hostBackups, func(i, j int) bool { return hostBackups[i].ModTime().Before(hostBackups[j].ModTime()) })
				
				var chains [][]os.FileInfo
				for _, backup := range hostBackups {
					isInc := strings.Contains(backup.Name(), "_INC_")
					if !isInc || len(chains) == 0 {
						chains = append(chains, []os.FileInfo{backup})
					} else {
						chains[len(chains)-1] = append(chains[len(chains)-1], backup)
					}
				}
				
				if len(chains) > job.ColdStorage.RetentionCount {
					numToDelete := len(chains) - job.ColdStorage.RetentionCount
					for i := 0; i < numToDelete; i++ {
						for _, oldBackup := range chains[i] {
							oldPath := filepath.Join(cleanDir, oldBackup.Name())
							os.Remove(oldPath)
							ui.Summary("   🧊🗑️  Pruned expired cold storage backup for %s: %s", job.Name, oldBackup.Name())
						}
					}
				}
			}
		}
	}
}


func SendNotification(configs []NotificationConfig, title, desc string, color int, hostName, targetFile string, ui tui.BackupUI) {
	// Dispatch to the multi-channel notification engine
	SendNotifications(configs, title, desc, hostName, targetFile, color)
}


func pushJobResult(server, jobName, status, errMsg, duration string) {
	StateMutex.Lock()
	defer StateMutex.Unlock()
	BatchResults = append(BatchResults, JobResult{
		Server: server,
		Job: jobName,
		Status: status,
		Error: errMsg,
		Duration: duration,
	})
}
