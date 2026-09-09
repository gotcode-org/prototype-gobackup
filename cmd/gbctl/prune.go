package main

import (
	"fmt"

	"gobackup/internal/engine"
	"gobackup/internal/tui"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Manually prune old backups according to retention policies",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := engine.LoadConfig("config.yaml")
		fmt.Printf("🧹 Manually pruning backups by count from %s...\n", cfg.BackupDir)
		engine.CleanupOldBackups(cfg.BackupDir, cfg.Hosts, tui.NewRawUI())
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)
}
