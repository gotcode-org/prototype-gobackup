package engine

import (
	"fmt"
	"log"

	"github.com/robfig/cron/v3"
	pb "gobackup/internal/grpc/pb"
)

type Scheduler struct {
	cron *cron.Cron
	cfg  Config
}

func NewScheduler(cfg Config) *Scheduler {
	return &Scheduler{
		cron: cron.New(),
		cfg:  cfg,
	}
}

// Start loads the hosts and starts the background cron engine
func (s *Scheduler) Start() {
	for _, host := range s.cfg.Hosts {
		if host.Schedule == "" {
			continue // No schedule defined, skip
		}
		
		// Capture the host variable for the closure
		targetHost := host
		
		_, err := s.cron.AddFunc(targetHost.Schedule, func() {
			log.Printf("⏰ CRON TRIGGERED: Starting scheduled backup for %s", targetHost.Name)
			
			// For now, use a basic Daemon logger that just prints to stdout.
			// In Phase 3, this will broadcast to connected gRPC clients!
			daemonUI := &DaemonLogger{hostName: targetHost.Name}
			
			// Run the backup for just this host
			RunSingleBackup(s.cfg, targetHost, daemonUI)
			
			log.Printf("✅ CRON FINISHED: Backup complete for %s", targetHost.Name)
		})
		
		if err != nil {
			log.Printf("❌ Failed to schedule backup for %s: %v", targetHost.Name, err)
		} else {
			log.Printf("📅 Scheduled %s -> %s", targetHost.Name, targetHost.Schedule)
		}
	}
	
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// --- Daemon Logger (Implements tui.BackupUI for background jobs) ---

type DaemonLogger struct {
	hostName string
}

func (d *DaemonLogger) Log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[Daemon UI - %s] %s", d.hostName, msg)
	GlobalLogBroker.Broadcast(&pb.LogChunk{Text: msg, IsSummary: false, HostName: d.hostName})
}
func (d *DaemonLogger) Summary(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[Daemon Summary - %s] %s", d.hostName, msg)
	GlobalLogBroker.Broadcast(&pb.LogChunk{Text: msg, IsSummary: true, HostName: d.hostName})
}
func (d *DaemonLogger) SetStatus(status string, spinning bool) {
	log.Printf("[Daemon Status - %s] %s", d.hostName, status)
	// Status updates are broadcast as summary lines for the TUI to render
	GlobalLogBroker.Broadcast(&pb.LogChunk{Text: status, IsStatus: true, Spinning: spinning, HostName: d.hostName})
}
func (d *DaemonLogger) Start() error { return nil }
func (d *DaemonLogger) Stop()        {}
