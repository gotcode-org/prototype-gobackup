package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"gobackup/internal/engine"
)

var port int

var rootCmd = &cobra.Command{
	Use:   "gobackupd",
	Short: "GoBackup Daemon Server",
	Long:  "gobackupd is the background gRPC daemon that schedules and executes backups.",
	Run: func(cmd *cobra.Command, args []string) {
		// Load the configuration first to get paths
		cfg := engine.LoadConfig("config.yaml")

		// Initialize the SQLite Identity Store using Config
		db, err := engine.InitDB(cfg.DBPath)
		if err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}

		// Boot up the native cron scheduler
		scheduler := engine.NewScheduler(cfg)
		scheduler.Start()
		defer scheduler.Stop()

		// Load or generate TLS certificates for encryption
		tlsCreds, err := engine.LoadOrGenerateTLS(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			log.Fatalf("Failed to initialize TLS: %v", err)
		}

		// Boot up the gRPC Server with TLS
		srv := engine.NewServer(cfg, db, scheduler)
		if err := srv.Start(port, tlsCreds); err != nil {
			log.Fatalf("Daemon crashed: %v", err)
		}
	},
}

func init() {
	rootCmd.Flags().IntVarP(&port, "port", "p", 50051, "Port to listen on")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
