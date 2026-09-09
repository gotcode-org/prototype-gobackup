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
		// Initialize the SQLite Identity Store
		db, err := engine.InitDB("gobackup.db")
		if err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}

		// Load the configuration
		cfg := engine.LoadConfig("config.yaml")

		// Boot up the native cron scheduler
		scheduler := engine.NewScheduler(cfg)
		scheduler.Start()
		defer scheduler.Stop()

		// Boot up the gRPC Server
		srv := engine.NewServer(db)
		if err := srv.Start(port); err != nil {
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
