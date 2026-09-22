package engine

import (
	"sync"
	"bytes"
	"encoding/json"
	"fmt"
	
	"net/http"
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

func EnqueueJob(host string) {
	StateMutex.Lock()
	defer StateMutex.Unlock()
	// Prevent duplicates so we can pre-populate global runs safely
	for _, v := range QueuedJobs {
		if v == host {
			return
		}
	}
	QueuedJobs = append(QueuedJobs, host)
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

func ClearActive() {
	StateMutex.Lock()
	defer StateMutex.Unlock()
	ActiveJob = ""
}

func RunBackups(cfg Config, ui tui.BackupUI) {
	for _, job := range cfg.Jobs {
		EnqueueJob(job.Name)
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
	
	EnqueueJob(job.Name)

	GlobalBackupQueue.Lock()
	DequeueAndSetActive(job.Name)

	defer func() {
		ClearActive()
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

	SendNotification(cfg.WebhookURL, 
		fmt.Sprintf("🔄 %s Backup Started (%s/%s)", backupType, job.Server, job.Name),
		fmt.Sprintf("Initiating tar pull natively for `%s/%s`.", job.Server, job.Name),
		3447003, job.Server, targetFile, ui)

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
		SendNotification(cfg.WebhookURL,
			fmt.Sprintf("❌ %s Backup Failed! (%s/%s)", backupType, job.Server, job.Name),
			fmt.Sprintf("Backup fatally failed after %s (Exit Code: %d).\\n\\n**Error Details:**\\n```text\\n%v\\n```", duration, exitCode, err),
			15158332, job.Server, targetFile, ui)

		ui.Summary("   ❌ %s Backup fatally failed for %s/%s (Exit Code %d): %v", backupType, job.Server, job.Name, exitCode, err)
		os.Remove(targetFile) 
		return
	}
	
	var sizeStr string
	if info, e := os.Stat(targetFile); e == nil {
		sizeStr = fmt.Sprintf("%.2f MB", float64(info.Size())/1024.0/1024.0)
	}

	if exitCode == 1 {
		SendNotification(cfg.WebhookURL,
			fmt.Sprintf("⚠️ %s Backup Completed with Warnings (%s/%s)", backupType, job.Server, job.Name),
			fmt.Sprintf("Archive finished in %s, but some active files changed or vanished during the backup process.\n\n**Statistics:**\n```text\nArchive Size: %s\n```", duration, sizeStr),
			16766720, job.Server, targetFile, ui)
		ui.Summary("   ⚠️  %s Completed with warnings (files changed) for %s/%s (%s)", backupType, job.Server, job.Name, sizeStr)
	} else {
		SendNotification(cfg.WebhookURL,
			fmt.Sprintf("✅ %s Backup Completed (%s/%s)", backupType, job.Server, job.Name),
			fmt.Sprintf("tar archive finished successfully in %s.\n\n**Statistics:**\n```text\nArchive Size: %s\n```", duration, sizeStr),
			3066993, job.Server, targetFile, ui)
		ui.Summary("   ✅ Success (%s)! Saved to %s (%s) in %s", backupType, targetFile, sizeStr, duration)
	}
}

func CleanupOldBackups(dir string, jobs []JobConfig, ui tui.BackupUI) {
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
						isDockerFile := len(parts) >= 5
						
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
							os.Remove(oldPath)
							ui.Summary("   🗑️  Pruned old backup for %s: %s", job.Name, oldBackup.Name())
						}
					}
				}
			}
		}
	}
}


func SendNotification(webhookURL, title, desc string, color int, hostName, targetFile string, ui tui.BackupUI) {
	if webhookURL == "" { return }
	
	payload := map[string]interface{}{
		"title":       title,
		"description": desc,
		"color":       color,
		"fields": [][]interface{}{
			{"Backup Server", "GoBackup-Daemon", true},
			{"Remote Target", hostName, true},
			{"Destination", targetFile, false},
		},
		"footer": "GoBackup Automated Task",
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", webhookURL, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		if ui != nil { ui.Log("   ❌ Webhook HTTP Error: %v", err) }
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		if ui != nil { ui.Log("   ❌ Webhook Rejected (HTTP %d): %s", resp.StatusCode, string(bodyBytes[:n])) }
	} else {
		if ui != nil { ui.Log("   ✅ Webhook Notification Sent") }
	}
}
