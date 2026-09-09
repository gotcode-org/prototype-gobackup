package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var serverAddr string

var loginCmd = &cobra.Command{
	Use:   "login [token]",
	Short: "Authenticate with the GoBackup daemon",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		token := args[0]
		
		cfg := LoadClientConfig()
		cfg.Token = token
		if serverAddr != "" {
			cfg.ServerAddress = serverAddr
		}

		if err := SaveClientConfig(cfg); err != nil {
			fmt.Printf("❌ Failed to save credentials: %v\n", err)
			return
		}

		fmt.Printf("✅ Successfully authenticated! Credentials saved.\n")
		fmt.Printf("🔌 Server Address: %s\n", cfg.ServerAddress)
	},
}

func init() {
	loginCmd.Flags().StringVarP(&serverAddr, "server", "s", "", "Remote daemon address (e.g. localhost:50051)")
	rootCmd.AddCommand(loginCmd)
}
