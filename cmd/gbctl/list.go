package main

import (
	"gobackup/internal/engine"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configuration or backups",
}

var listServersCmd = &cobra.Command{
	Use:   "servers",
	Short: "List configured servers",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := engine.LoadConfig("config.yaml")
		engine.ListServers(cfg)
	},
}

var listBackupsCmd = &cobra.Command{
	Use:   "backups [server]",
	Short: "List available backups",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := engine.LoadConfig("config.yaml")
		serverName := ""
		if len(args) == 1 {
			serverName = args[0]
		}
		engine.ListBackups(cfg, serverName)
	},
}

func init() {
	listCmd.AddCommand(listServersCmd)
	listCmd.AddCommand(listBackupsCmd)
	rootCmd.AddCommand(listCmd)
}
