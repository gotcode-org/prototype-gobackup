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

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Manually trigger the daemon to clean up old backups",
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

		resp, err := client.PruneBackups(context.Background(), &pb.PruneRequest{})
		if err != nil {
			log.Fatalf("❌ RPC Error: %v", err)
		}

		if resp.Success {
			fmt.Printf("✅ Daemon responded: %s\n", resp.Message)
		}
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)
}
