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
	for _, job := range s.cfg.Jobs {
		if job.Schedule == "" {
			continue // No schedule defined, skip
		}
		
		// Capture variable for the goroutine closure
		j := job
		_, err := s.cron.AddFunc(j.Schedule, func() {
			daemonUI := &DaemonLogger{hostName: j.Name}
			RunSingleBackup(s.cfg, j, daemonUI)
		})
		
		if err != nil {
			log.Printf("❌ Failed to schedule job %s: %v", j.Name, err)
		} else {
			log.Printf("🗓️  Scheduled %s for %s", j.Name, j.Schedule)
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
