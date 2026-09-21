package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gobackup/internal/grpc/pb"
)

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove resources from the GoBackup daemon",
}

var rmHostCmd = &cobra.Command{
	Use:   "host [name]",
	Short: "Remove a host from the configuration",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
			grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token}),
		}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		
		client := pb.NewAdminServiceClient(conn)
		resp, err := client.RemoveHost(context.Background(), &pb.RemoveHostRequest{Name: args[0]})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		fmt.Printf("✅ %s\n", resp.Message)
	},
}

var rmBackupCmd = &cobra.Command{
	Use:   "backup [filename]",
	Short: "Delete a specific backup file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
			grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token}),
		}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		
		// Wait, RemoveBackup is in AdminService
		client := pb.NewAdminServiceClient(conn)
		resp, err := client.RemoveBackup(context.Background(), &pb.RemoveBackupRequest{Filename: args[0]})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		fmt.Printf("✅ %s\n", resp.Message)
	},
}

func init() {
	rmCmd.AddCommand(rmHostCmd)
	rmCmd.AddCommand(rmBackupCmd)
	rootCmd.AddCommand(rmCmd)
}
