package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"crypto/tls"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gobackup/internal/grpc/pb"
)

var (
	schedule   string
	paths      string
	retention  int
	useSudo    bool
	sshPort    int
	group      string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add resources to the GoBackup daemon",
}

var addHostCmd = &cobra.Command{
	Use:   "host [name] [address]",
	Short: "Dynamically configure and schedule a new backup host",
	Args:  cobra.ExactArgs(2),
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
		
		// Note: AddHost is an AdminService RPC
		client := pb.NewAdminServiceClient(conn)

		pathList := strings.Split(paths, ",")
		if len(pathList) == 1 && pathList[0] == "" {
			pathList = []string{}
		}

		req := &pb.AddHostRequest{
			Name:           args[0],
			Address:        args[1],
			Schedule:       schedule,
			RetentionCount: int32(retention),
			UseSudo:        useSudo,
			Port:        int32(sshPort),
			Group:          group,
			Paths:          pathList,
		}

		resp, err := client.AddHost(context.Background(), req)
		if err != nil {
			log.Fatalf("❌ RPC Error: %v", err)
		}

		fmt.Printf("✅ %s\n", resp.Message)
	},
}

func init() {
	addHostCmd.Flags().StringVarP(&schedule, "schedule", "s", "", "Cron schedule (e.g., '0 2 * * *')")
	addHostCmd.Flags().StringVarP(&paths, "paths", "p", "/etc,/var/www", "Comma-separated paths to backup")
	addHostCmd.Flags().IntVarP(&retention, "retention", "r", 5, "Number of backups to keep")
	addHostCmd.Flags().BoolVar(&useSudo, "sudo", false, "Use sudo for remote tar execution")
	addHostCmd.Flags().IntVar(&sshPort, "port", 22, "SSH port")
	addHostCmd.Flags().StringVarP(&group, "group", "g", "servers", "Host group category")
	
	addCmd.AddCommand(addHostCmd)
	rootCmd.AddCommand(addCmd)
}
