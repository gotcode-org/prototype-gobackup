package main

import (
	"log"
	"os"

	"gobackup/internal/engine"
	"gobackup/internal/tui"
	"github.com/spf13/cobra"
)

var useCron bool

var runCmd = &cobra.Command{
	Use:   "run [server/group]",
	Short: "Run backups locally",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := engine.LoadConfig("config.yaml")

		if len(args) == 1 {
			targetArg := args[0]
			var filtered []engine.HostConfig
			for _, h := range cfg.Hosts {
				if h.Name == targetArg || h.Group == targetArg {
					filtered = append(filtered, h)
				}
			}
			if len(filtered) == 0 {
				log.Fatalf("❌ Error: No server or group named '%s' found in config.yaml\n", targetArg)
			}
			cfg.Hosts = filtered
		}

		if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
			useCron = true
		}

		var ui tui.BackupUI
		if useCron {
			ui = tui.NewRawUI()
		} else {
			ui = tui.NewTviewUI()
		}

		go func() {
			engine.RunBackups(cfg, ui)
			ui.Stop()
		}()

		if err := ui.Start(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	runCmd.Flags().BoolVarP(&useCron, "cron", "c", false, "Run in background cron mode (raw output)")
	rootCmd.AddCommand(runCmd)
}
