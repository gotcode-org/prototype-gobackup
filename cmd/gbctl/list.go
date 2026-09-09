package main

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gobackup/internal/grpc/pb"
	"crypto/tls"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured backup hosts via daemon",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		if cfg.Token == "" {
			log.Fatalf("❌ Not authenticated. Please run: gbctl login <TOKEN>")
		}

		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
			grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token}),
		}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil {
			log.Fatalf("❌ Failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewBackupServiceClient(conn)

		resp, err := client.ListHosts(context.Background(), &pb.ListRequest{})
		if err != nil {
			log.Fatalf("❌ RPC Error: %v", err)
		}

		fmt.Println("\n🗄️  Configured Backup Hosts (From Daemon):")
		fmt.Println("-----------------------------------------------------")
		for _, h := range resp.Hosts {
			schedule := h.Schedule
			if schedule == "" { schedule = "Manual Only" }
			fmt.Printf("📦 %s\n", h.Name)
			fmt.Printf("   Address:   %s\n", h.Address)
			fmt.Printf("   Schedule:  %s\n", schedule)
			fmt.Printf("   Retention: %d copies\n\n", h.RetentionCount)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
