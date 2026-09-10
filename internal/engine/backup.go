package engine

import (
	"sync"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
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

func ListServers(cfg Config) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
	fmt.Fprintln(w, "NAME\tGROUP\tADDRESS\tPORT\tPATHS")
	for _, host := range cfg.Hosts {
		port := host.Port
		if port == 0 {
			port = 22
		}
		paths := strings.Join(host.Paths, ", ")
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", host.Name, host.Group, host.Address, port, paths)
	}
	w.Flush()
}

func ListBackups(cfg Config, serverName string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
	fmt.Fprintln(w, "NAME\tSIZE\tCREATED AT\tAGE")

	files, err := os.ReadDir(cfg.BackupDir)
	if err != nil {
		log.Fatalf("❌ Failed to read backup directory: %v", err)
	}

	prefix := serverName + "_"
	found := false

	for _, entry := range files {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}
		if serverName != "" && !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}

		found = true
		sizeMB := float64(info.Size()) / 1024.0 / 1024.0
		age := time.Since(info.ModTime())
		
		ageStr := ""
		if age.Hours() > 24 {
			ageStr = fmt.Sprintf("%dd", int(age.Hours()/24))
		} else if age.Hours() >= 1 {
			ageStr = fmt.Sprintf("%dh", int(age.Hours()))
		} else {
			ageStr = fmt.Sprintf("%dm", int(age.Minutes()))
		}

		fmt.Fprintf(w, "%s\t%.2f MB\t%s\t%s\n", 
			entry.Name(), sizeMB, info.ModTime().Format("2006-01-02 15:04:05"), ageStr)
	}
	w.Flush()
	if !found {
		fmt.Println("No backups found.")
	}
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
	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		ui.Log("❌ Failed to create backup directory %s: %v", cfg.BackupDir, err)
		return
	}

	for _, host := range cfg.Hosts {
		RunSingleBackup(cfg, host, ui)
	}

	ui.SetStatus("Running Retention Cleanup...", true)
	ui.Summary("🧹 Cleaning up old backups...")
	CleanupOldBackups(cfg.BackupDir, cfg.Hosts, ui)
	ui.SetStatus("Backup Complete!", false)
	ui.Summary("🎉 Backup job complete!")
}

func RunSingleBackup(cfg Config, host HostConfig, ui tui.BackupUI) {
	ui.SetStatus(fmt.Sprintf("Queued: %s", host.Name), true)
	ui.Log("⏳ Job for %s entered the global queue. Waiting for active jobs to finish...", host.Name)
	
	EnqueueJob(host.Name)

	GlobalBackupQueue.Lock()
	DequeueAndSetActive(host.Name)

	defer func() {
		ClearActive()
		GlobalBackupQueue.Unlock()
	}()

	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		ui.Log("❌ Failed to create backup directory %s: %v", cfg.BackupDir, err)
		return
	}

	if host.Port == 0 {
		host.Port = 22
	}

	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%s.tar.gz", host.Name, timestamp)
	targetFile := filepath.Join(cfg.BackupDir, fileName)

	var cmd *exec.Cmd
	if host.Address == "localhost" || host.Address == "127.0.0.1" || host.Address == "local" {
		var args []string
		if host.UseSudo {
			args = []string{"tar", "-cvzf", "-"}
			args = append(args, host.Paths...)
			cmd = exec.Command("sudo", args...)
		} else {
			args = []string{"-cvzf", "-"}
			args = append(args, host.Paths...)
			cmd = exec.Command("tar", args...)
		}
	} else {
		args := []string{"-p", strconv.Itoa(host.Port), host.Address}
		if host.UseSudo {
			args = append(args, "sudo", "-n", "tar", "-cvzf", "-")
		} else {
			args = append(args, "tar", "-cvzf", "-")
		}
		args = append(args, host.Paths...)
		cmd = exec.Command("ssh", args...)
	}
	
	outFile, err := os.Create(targetFile)
	if err != nil {
		ui.Log("   ❌ Error creating local file: %v", err)
		return
	}
	cmd.Stdout = outFile
	cmd.Stderr = &cmdLogger{ui: ui} 

	SendNotification(cfg.WebhookURL, 
		fmt.Sprintf("🔄 Backup Started (%s)", host.Name),
		fmt.Sprintf("Initiating tar pull natively for `%s`.", host.Name),
		3447003, host.Name, targetFile, ui)

	startTime := time.Now()

	ui.SetStatus(fmt.Sprintf("Backing up host: %s (%s)", host.Name, host.Address), true)

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
			fmt.Sprintf("❌ Backup Failed! (%s)", host.Name),
			fmt.Sprintf("Backup fatally failed after %s (Exit Code: %d).\n\n**Error Details:**\n```text\n%v\n```", duration, exitCode, err),
			15158332, host.Name, targetFile, ui)

		ui.Summary("   ❌ Backup fatally failed for %s (Exit Code %d): %v", host.Name, exitCode, err)
		os.Remove(targetFile) 
		return
	}
	
	var sizeStr string
	if info, e := os.Stat(targetFile); e == nil {
		sizeStr = fmt.Sprintf("%.2f MB", float64(info.Size())/1024.0/1024.0)
	}

	if exitCode == 1 {
		SendNotification(cfg.WebhookURL,
			fmt.Sprintf("⚠️ Backup Completed with Warnings (%s)", host.Name),
			fmt.Sprintf("Archive finished in %s, but some active files changed or vanished during the backup process.\n\n**Statistics:**\n```text\nArchive Size: %s\n```", duration, sizeStr),
			16766720, host.Name, targetFile, ui)
		ui.Summary("   ⚠️  Completed with warnings (files changed) for %s (%s)", host.Name, sizeStr)
	} else {
		SendNotification(cfg.WebhookURL,
			fmt.Sprintf("✅ Backup Completed (%s)", host.Name),
			fmt.Sprintf("tar archive finished successfully in %s.\n\n**Statistics:**\n```text\nArchive Size: %s\n```", duration, sizeStr),
			3066993, host.Name, targetFile, ui)
		ui.Summary("   ✅ Success! Saved to %s (%s) in %s", targetFile, sizeStr, duration)
	}
	
	// Prune just this host after it finishes
	CleanupOldBackups(cfg.BackupDir, []HostConfig{host}, ui)
}

func CleanupOldBackups(dir string, hosts []HostConfig, ui tui.BackupUI) {
	files, err := os.ReadDir(dir)
	if err != nil {
		ui.Log("❌ Failed to read backup directory for cleanup: %v", err)
		return
	}

	for _, host := range hosts {
		if host.RetentionCount <= 0 {
			continue 
		}

		var hostBackups []os.FileInfo
		for _, entry := range files {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
				continue
			}
			if prefix := host.Name + "_"; strings.HasPrefix(entry.Name(), prefix) {
				info, err := entry.Info()
				if err == nil {
					hostBackups = append(hostBackups, info)
				}
			}
		}

		sort.Slice(hostBackups, func(i, j int) bool {
			return hostBackups[i].ModTime().After(hostBackups[j].ModTime())
		})

		if len(hostBackups) > host.RetentionCount {
			for _, oldBackup := range hostBackups[host.RetentionCount:] {
				oldPath := filepath.Join(dir, oldBackup.Name())
				os.Remove(oldPath)
				ui.Summary("   🗑️  Pruned old backup for %s: %s", host.Name, oldBackup.Name())
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
	client := &http.Client{}
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
