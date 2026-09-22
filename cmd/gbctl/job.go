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
	jobServer       string
	jobSchedule     string
	jobRetention    int
	jobPaths        string
	jobVolumes      string
	jobPause        string
	jobIncremental  bool
	jobFullInterval int
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
		req := &pb.AddJobRequest{Name: args[0], Server: jobServer, Schedule: jobSchedule, RetentionCount: int32(jobRetention), Incremental: jobIncremental, FullInterval: int32(jobFullInterval)}
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
		fmt.Fprintln(w, "SERVER\tJOB\tSCHEDULE\tRETENTION")
		fmt.Fprintln(w, "------\t---\t--------\t---------")
		for _, j := range resp.Jobs {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", j.Server, j.Name, j.Schedule, j.RetentionCount)
		}
		w.Flush()
	},
}

var jobRmCmd = &cobra.Command{
	Use:   "rm [server] [name]",
	Short: "Remove a job",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewAdminServiceClient(conn)
		resp, err := client.RemoveJob(context.Background(), &pb.RemoveJobRequest{Server: args[0], Name: args[1]})
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
	jobAddCmd.Flags().BoolVar(&jobIncremental, "incremental", false, "Enable incremental backups (tar -g)")
	jobAddCmd.Flags().IntVar(&jobFullInterval, "full-interval", 7, "Days between FULL backups when incremental is enabled")
	jobRunCmd.Flags().BoolVar(&jobRunAttach, "attach", false, "Attach to the log stream immediately")
	
	jobCmd.AddCommand(jobAddCmd, jobListCmd, jobRmCmd, jobRunCmd)
	rootCmd.AddCommand(jobCmd)
}

var jobRunAttach bool
var jobRunCmd = &cobra.Command{
	Use:   "run [server] [name]",
	Short: "Trigger backup jobs asynchronously",
	Long:  "Usage:\n  gbctl job run                     (Run all jobs)\n  gbctl job run <server>            (Run all jobs for server)\n  gbctl job run <server> <job>      (Run specific job)",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewBackupServiceClient(conn)
		
		req := &pb.BackupRequest{}
		if len(args) >= 1 { req.TargetServer = args[0] }
		if len(args) == 2 { req.TargetJob = args[1] }
		
		resp, err := client.StartBackup(context.Background(), req)
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		
		fmt.Printf("✅ %s\n", resp.Message)
		if jobRunAttach {
			watchReq := &pb.WatchRequest{}
			if len(args) == 2 { watchReq.JobId = args[1] }
			stream, err := client.WatchLogs(context.Background(), watchReq)
			if err != nil { log.Fatalf("❌ Stream Error: %v", err) }
			for {
				chunk, err := stream.Recv()
				if err != nil { break }
				fmt.Print(chunk.Text)
			}
		} else {
			fmt.Println("To watch the live log stream, run: gbctl attach")
		}
	},
}
