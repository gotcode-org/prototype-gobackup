package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gobackupd",
	Short: "GoBackup Daemon Server",
	Long:  "gobackupd is the background gRPC daemon that schedules and executes backups.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 GoBackup Daemon is running (Stub)")
		// Future: load config, start grpc server, start cron scheduler, block forever.
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
