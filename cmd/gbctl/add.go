package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gobackup/internal/grpc/pb"
)

var (
	addGroup      string
	addPort       int
	addSudo       bool
	addSchedule   string
	addRetention  int
	addPaths      string
	addVolumes    string
	addPause      string
	addJobServer  string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add resources to the GoBackup daemon",
}

var addServerCmd = &cobra.Command{
	Use:   "server [name] [address]",
	Short: "Add a new server configuration",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		req := &pb.AddServerRequest{
			Name:    args[0],
			Address: args[1],
			Group:   addGroup,
			Port:    int32(addPort),
			UseSudo: addSudo,
		}
		sendAddServerRequest(req)
	},
}

var addJobCmd = &cobra.Command{
	Use:   "job [name]",
	Short: "Add a new backup job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		req := &pb.AddJobRequest{
			Name:           args[0],
			Server:         addJobServer,
			Schedule:       addSchedule,
			RetentionCount: int32(addRetention),
		}
		if addPaths != "" {
			req.Paths = strings.Split(addPaths, ",")
		}
		if addVolumes != "" {
			req.DockerVolumes = strings.Split(addVolumes, ",")
		}
		if addPause != "" {
			req.PauseContainers = strings.Split(addPause, ",")
		}
		sendAddJobRequest(req)
	},
}

func sendAddServerRequest(req *pb.AddServerRequest) {
	cfg := LoadClientConfig()
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
		grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token}),
	}
	conn, err := grpc.Dial(cfg.ServerAddress, opts...)
	if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
	defer conn.Close()
	
	client := pb.NewAdminServiceClient(conn)
	resp, err := client.AddServer(context.Background(), req)
	if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
	fmt.Printf("✅ %s\n", resp.Message)
}

func sendAddJobRequest(req *pb.AddJobRequest) {
	cfg := LoadClientConfig()
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
		grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token}),
	}
	conn, err := grpc.Dial(cfg.ServerAddress, opts...)
	if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
	defer conn.Close()
	
	client := pb.NewAdminServiceClient(conn)
	resp, err := client.AddJob(context.Background(), req)
	if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
	fmt.Printf("✅ %s\n", resp.Message)
}

func init() {
	addServerCmd.Flags().StringVarP(&addGroup, "group", "g", "default", "Server group name")
	addServerCmd.Flags().IntVarP(&addPort, "port", "p", 22, "SSH Port")
	addServerCmd.Flags().BoolVar(&addSudo, "sudo", false, "Use sudo for execution")
	
	addJobCmd.Flags().StringVar(&addJobServer, "server", "", "Target server for this job (required)")
	addJobCmd.MarkFlagRequired("server")
	addJobCmd.Flags().StringVarP(&addSchedule, "schedule", "s", "", "Cron schedule")
	addJobCmd.Flags().IntVarP(&addRetention, "retention", "r", 7, "Number of backups to keep")
	addJobCmd.Flags().StringVar(&addPaths, "paths", "", "Comma-separated list of paths to backup")
	addJobCmd.Flags().StringVar(&addVolumes, "volumes", "", "Comma-separated list of Docker volumes to backup")
	addJobCmd.Flags().StringVar(&addPause, "pause", "", "Comma-separated list of Docker containers to pause during backup")

	addCmd.AddCommand(addServerCmd)
	addCmd.AddCommand(addJobCmd)
	rootCmd.AddCommand(addCmd)
}
