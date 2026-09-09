package main

import (
	"context"
	"fmt"
	"log"
	"crypto/tls"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gobackup/internal/grpc/pb"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configurations or backup archives",
}

var listHostsCmd = &cobra.Command{
	Use:     "servers",
	Aliases: []string{"hosts"},
	Short:   "List configured backup servers",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		if cfg.Token == "" {
			log.Fatalf("❌ Not authenticated.")
		}

		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
			grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token}),
		}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		
		client := pb.NewBackupServiceClient(conn)
		resp, err := client.ListHosts(context.Background(), &pb.ListRequest{})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }

		fmt.Println("\n🗄️  Configured Backup Hosts:")
		fmt.Println("-----------------------------------------------------")
		for _, h := range resp.Hosts {
			schedule := h.Schedule
			if schedule == "" { schedule = "Manual Only" }
			fmt.Printf("📦 %s\n   Address:   %s\n   Schedule:  %s\n   Retention: %d copies\n\n", h.Name, h.Address, schedule, h.RetentionCount)
		}
	},
}

var listBackupsCmd = &cobra.Command{
	Use:   "backups [host]",
	Short: "List actual backup archives stored on disk",
	Run: func(cmd *cobra.Command, args []string) {
		target := ""
		if len(args) > 0 { target = args[0] }

		cfg := LoadClientConfig()
		if cfg.Token == "" {
			log.Fatalf("❌ Not authenticated.")
		}

		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
			grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token}),
		}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		
		client := pb.NewBackupServiceClient(conn)
		resp, err := client.ListBackups(context.Background(), &pb.ListBackupsRequest{Target: target})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }

		fmt.Println("\n💾 Backup Archives on Disk:")
		fmt.Println("-----------------------------------------------------")
		for _, a := range resp.Archives {
			sizeMB := float64(a.Size) / 1024 / 1024
			fmt.Printf("[%s] %s | %.2f MB | %s\n", a.Host, a.Filename, sizeMB, a.Modified)
		}
		if len(resp.Archives) == 0 {
			fmt.Println("No backups found.")
		}
		fmt.Println()
	},
}

func init() {
	listCmd.AddCommand(listHostsCmd)
	listCmd.AddCommand(listBackupsCmd)
	rootCmd.AddCommand(listCmd)
}
