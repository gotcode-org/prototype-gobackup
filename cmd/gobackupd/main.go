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
		srv := engine.NewServer()
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
