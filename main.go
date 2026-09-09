package main

import (
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

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"gopkg.in/yaml.v3"
)

// --- Config Structs ---
type Config struct {
	BackupDir  string       `yaml:"backup_dir"`
	WebhookURL string       `yaml:"webhook_url"`
	Hosts      []HostConfig `yaml:"hosts"`
}

type HostConfig struct {
	Name           string   `yaml:"name"`
	Group          string   `yaml:"group"`
	Address        string   `yaml:"address"`
	Port           int      `yaml:"port"`
	UseSudo        bool     `yaml:"use_sudo"`
	RetentionCount int      `yaml:"retention_count"`
	Paths          []string `yaml:"paths"`
}

// --- UI Interface ---
type BackupUI interface {
	Log(format string, args ...interface{})
	Summary(format string, args ...interface{})
	SetStatus(status string, spinning bool)
	Start() error
	Stop()
}

// --- Raw UI (For --cron) ---
type RawUI struct{
	done chan bool
}
func NewRawUI() *RawUI { return &RawUI{done: make(chan bool)} }

func (u *RawUI) Log(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}
func (u *RawUI) Summary(format string, args ...interface{}) { fmt.Printf(format+"\n", args...) }
func (u *RawUI) SetStatus(status string, spinning bool) {
	fmt.Printf("\n>>> %s\n", status)
}
func (u *RawUI) Start() error { <-u.done; return nil }
func (u *RawUI) Stop()        { close(u.done) }

// --- Terminal UI (For Humans) ---
type TviewUI struct {
	app          *tview.Application
	statusView   *tview.TextView
	logView      *tview.TextView
	statusText   string
	isSpinning   bool
	stopSpinner  chan bool
	summaryLines []string
}

func NewTviewUI() *TviewUI {
	app := tview.NewApplication()

	statusView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	statusView.SetBorder(true).SetTitle(" Status ")

	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetMaxLines(1000)
	logView.SetBorder(true).SetTitle(" Real-Time Logs ")
	logView.SetChangedFunc(func() { app.Draw() })

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statusView, 16, 1, false).
		AddItem(logView, 0, 3, false)

	app.SetRoot(flex, true)

	ui := &TviewUI{
		app:         app,
		statusView:  statusView,
		logView:     logView,
		stopSpinner: make(chan bool),
	}
	ui.startSpinnerLoop()
	return ui
}

func (u *TviewUI) Log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// Escape the string to prevent tview from confusing raw logs with color tags
	safeMsg := tview.Escape(msg)
	
	// Guarantee the write and the redraw happen flawlessly on the main UI thread
	u.app.QueueUpdateDraw(func() {
		fmt.Fprintf(u.logView, "%s\n", safeMsg)
		u.logView.ScrollToEnd()
	})
}
func (u *TviewUI) Summary(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	u.app.QueueUpdateDraw(func() {
		u.summaryLines = append(u.summaryLines, msg)
		if len(u.summaryLines) > 10 {
			u.summaryLines = u.summaryLines[len(u.summaryLines)-10:]
		}
	})
	// also duplicate to raw logs for history
	u.Log(msg)
}

func (u *TviewUI) SetStatus(status string, spinning bool) {
	u.statusText = status
	u.isSpinning = spinning
}

func (u *TviewUI) startSpinnerLoop() {
	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	go func() {
		for {
			select {
			case <-u.stopSpinner:
				return
			default:
				if u.statusText != "" {
					var text string
					
					history := strings.Join(u.summaryLines, "\n")
					if history != "" {
						history = "\n\n" + history
					}
					if u.isSpinning {
						text = fmt.Sprintf("[yellow::b]%s %s[-::-]%s", spinChars[i], u.statusText, history)
						i = (i + 1) % len(spinChars)
					} else {
						text = fmt.Sprintf("[green::b]%s[-::-]%s", u.statusText, history)
					}
					u.app.QueueUpdateDraw(func() {
						u.statusView.SetText(text)
					})
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}
func (u *TviewUI) Start() error { return u.app.Run() }
func (u *TviewUI) Stop() {
	u.app.QueueUpdateDraw(func() {
		u.statusView.SetText(u.statusView.GetText(false) + "\n\n[white::b]Backup finished! Press 'q', 'Enter', or 'Esc' to exit...[-::-]")
	})
	u.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyEscape {
			u.app.Stop()
		}
		return event
	})
}

// --- Core Logic ---
func main() {
	// Parse out --cron and clean args
	useCron := false
	var cleanArgs []string
	for _, arg := range os.Args {
		if arg == "--cron" {
			useCron = true
		} else {
			cleanArgs = append(cleanArgs, arg)
		}
	}

	// Auto-detect if stdout is piped/redirected
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		useCron = true
	}

	cfg := loadConfig()

	// Parse subcommands
	if len(cleanArgs) > 1 {
		switch cleanArgs[1] {
		case "list":
			if len(cleanArgs) < 3 {
				fmt.Println("Usage: ./gobackup list servers OR ./gobackup list backups <server>")
				os.Exit(1)
			}
			if cleanArgs[2] == "servers" {
				listServers(cfg)
				return
			} else if cleanArgs[2] == "backups" {
				serverName := ""
				if len(cleanArgs) >= 4 {
					serverName = cleanArgs[3]
				}
				listBackups(cfg, serverName)
				return
			}
		case "prune":
			fmt.Printf("🧹 Manually pruning backups by count from %s...\n", cfg.BackupDir)
			cleanupOldBackups(cfg.BackupDir, cfg.Hosts, NewRawUI())
			return
		default:
			// Targeted backup for a specific server OR group
			targetArg := cleanArgs[1]
			var filtered []HostConfig
			for _, h := range cfg.Hosts {
				if h.Name == targetArg || h.Group == targetArg {
					filtered = append(filtered, h)
				}
			}
			if len(filtered) == 0 {
				fmt.Printf("❌ Error: No server or group named '%s' found in config.yaml\n", targetArg)
				os.Exit(1)
			}
			cfg.Hosts = filtered
		}
	}

	// Determine UI
	var ui BackupUI
	if useCron {
		ui = NewRawUI()
	} else {
		ui = NewTviewUI()
	}

	// Run backup in background so UI can block main thread
	go func() {
		runBackups(cfg, ui)
		ui.Stop()
	}()

	// Start UI (blocks until ui.Stop() is called)
	if err := ui.Start(); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() Config {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("❌ Failed to read config.yaml: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("❌ Failed to parse config.yaml: %v", err)
	}
	return cfg
}

func listServers(cfg Config) {
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

func listBackups(cfg Config, serverName string) {
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

func runBackups(cfg Config, ui BackupUI) {
	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		ui.Log("❌ Failed to create backup directory %s: %v", cfg.BackupDir, err)
		return
	}

	ui.Summary("🚀 Starting GoBackup...")

	for _, host := range cfg.Hosts {
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
			continue
		}
		cmd.Stdout = outFile
		cmd.Stderr = &cmdLogger{ui: ui} 

		sendNotification(cfg.WebhookURL, 
			fmt.Sprintf("🔄 Backup Started (%s)", host.Name),
			fmt.Sprintf("Initiating tar pull natively for `%s`.", host.Name),
			3447003, host.Name, targetFile)

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

		// Exit Code 1 means "Some files differ" (e.g., file changed while reading). This is a warning, not a failure.
		if exitCode != 0 && exitCode != 1 {
			sendNotification(cfg.WebhookURL,
				fmt.Sprintf("❌ Backup Failed! (%s)", host.Name),
				fmt.Sprintf("Backup fatally failed after %s (Exit Code: %d).\n\n**Error Details:**\n```text\n%v\n```", duration, exitCode, err),
				15158332, host.Name, targetFile)

			ui.Summary("   ❌ Backup fatally failed for %s (Exit Code %d): %v", host.Name, exitCode, err)
			os.Remove(targetFile) // Delete the corrupted archive
			continue
		}
		
		var sizeStr string
		if info, e := os.Stat(targetFile); e == nil {
			sizeStr = fmt.Sprintf("%.2f MB", float64(info.Size())/1024.0/1024.0)
		}

		if exitCode == 1 {
			// Yellow Warning Notification (Color: 16766720)
			sendNotification(cfg.WebhookURL,
				fmt.Sprintf("⚠️ Backup Completed with Warnings (%s)", host.Name),
				fmt.Sprintf("Archive finished in %s, but some active files changed or vanished during the backup process.\n\n**Statistics:**\n```text\nArchive Size: %s\n```", duration, sizeStr),
				16766720, host.Name, targetFile)
			ui.Summary("   ⚠️  Completed with warnings (files changed) for %s (%s)", host.Name, sizeStr)
		} else {
			// Green Success Notification (Color: 3066993)
			sendNotification(cfg.WebhookURL,
				fmt.Sprintf("✅ Backup Completed (%s)", host.Name),
				fmt.Sprintf("tar archive finished successfully in %s.\n\n**Statistics:**\n```text\nArchive Size: %s\n```", duration, sizeStr),
				3066993, host.Name, targetFile)
			ui.Summary("   ✅ Success! Saved to %s (%s) in %s", targetFile, sizeStr, duration)
		}
	}

	ui.SetStatus("Running Retention Cleanup...", true)
	ui.Summary("🧹 Cleaning up old backups...")
	cleanupOldBackups(cfg.BackupDir, cfg.Hosts, ui)
	ui.SetStatus("Backup Complete!", false)
	ui.Summary("🎉 Backup job complete!")
	
}

func cleanupOldBackups(dir string, hosts []HostConfig, ui BackupUI) {
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

func sendNotification(webhookURL, title, desc string, color int, hostName, targetFile string) {
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
	client.Do(req)
}

type cmdLogger struct {
	ui BackupUI
}

func (c *cmdLogger) Write(p []byte) (n int, err error) {
	// Write the entire chunk at once to prevent flooding the TUI event loop
	c.ui.Log("%s", strings.TrimSpace(string(p)))
	return len(p), nil
}
