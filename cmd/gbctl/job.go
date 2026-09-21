package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	pb "gobackup/internal/grpc/pb"
)

var (
	jobServer    string
	jobSchedule  string
	jobRetention int
	jobPaths     string
	jobVolumes   string
	jobPause     string
)

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage backup jobs",
}

var jobAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		req := &pb.AddJobRequest{Name: args[0], Server: jobServer, Schedule: jobSchedule, RetentionCount: int32(jobRetention)}
		if jobPaths != "" { req.Paths = strings.Split(jobPaths, ",") }
		if jobVolumes != "" { req.DockerVolumes = strings.Split(jobVolumes, ",") }
		if jobPause != "" { req.PauseContainers = strings.Split(jobPause, ",") }
		
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewAdminServiceClient(conn)
		resp, err := client.AddJob(context.Background(), req)
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		fmt.Printf("✅ %s\n", resp.Message)
	},
}

var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List jobs",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewBackupServiceClient(conn)
		resp, err := client.ListJobs(context.Background(), &pb.ListRequest{})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		if len(resp.Jobs) == 0 { fmt.Println("No jobs configured."); return }
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
		fmt.Fprintln(w, "JOB\tSERVER\tSCHEDULE\tRETENTION")
		fmt.Fprintln(w, "---\t------\t--------\t---------")
		for _, j := range resp.Jobs {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", j.Name, j.Server, j.Schedule, j.RetentionCount)
		}
		w.Flush()
	},
}

var jobRmCmd = &cobra.Command{
	Use:   "rm [name]",
	Short: "Remove a job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewAdminServiceClient(conn)
		resp, err := client.RemoveJob(context.Background(), &pb.RemoveJobRequest{Name: args[0]})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		fmt.Printf("✅ %s\n", resp.Message)
	},
}

func init() {
	jobAddCmd.Flags().StringVar(&jobServer, "server", "", "Target server (required)")
	jobAddCmd.MarkFlagRequired("server")
	jobAddCmd.Flags().StringVarP(&jobSchedule, "schedule", "s", "", "Cron schedule")
	jobAddCmd.Flags().IntVarP(&jobRetention, "retention", "r", 7, "Number of backups to keep")
	jobAddCmd.Flags().StringVar(&jobPaths, "paths", "", "Comma-separated list of paths to backup")
	jobAddCmd.Flags().StringVar(&jobVolumes, "volumes", "", "Comma-separated list of Docker volumes to backup")
	jobAddCmd.Flags().StringVar(&jobPause, "pause", "", "Comma-separated list of Docker containers to pause")
	
	jobCmd.AddCommand(jobAddCmd, jobListCmd, jobRmCmd, jobRunCmd)
	rootCmd.AddCommand(jobCmd)
}

var jobRunCmd = &cobra.Command{
	Use:   "run [name]",
	Short: "Trigger a backup job asynchronously",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewBackupServiceClient(conn)
		
		resp, err := client.StartBackup(context.Background(), &pb.BackupRequest{Target: args[0]})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		
		fmt.Printf("✅ %s\n", resp.Message)
		fmt.Println("To watch the live log stream, run: gbctl attach")
	},
}
