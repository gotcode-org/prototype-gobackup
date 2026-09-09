package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gbctl",
	Short: "GoBackup Client/TUI",
	Long:  "gbctl is the client for GoBackup. Currently acts as a local standalone runner until gRPC is fully implemented.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
