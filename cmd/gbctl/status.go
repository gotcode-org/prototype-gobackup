package main

import (
	"context"
	"fmt"
	"log"
	"crypto/tls"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gobackup/internal/grpc/pb"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the live status of the daemon and global backup queue",
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
		if err != nil {
			log.Fatalf("❌ Daemon is OFFLINE (Failed to connect: %v)", err)
		}
		defer conn.Close()
		
		client := pb.NewBackupServiceClient(conn)
		resp, err := client.GetStatus(context.Background(), &pb.StatusRequest{})
		if err != nil {
			log.Fatalf("❌ RPC Error: %v", err)
		}

		fmt.Println("\n🟢 Daemon Status: ONLINE")
		fmt.Println("-----------------------------------------------------")
		
		if resp.ActiveJob != "" {
			fmt.Printf("🔥 Active Job: %s\n", resp.ActiveJob)
		} else {
			fmt.Println("💤 Active Job: None (Idle)")
		}

		if len(resp.QueuedJobs) > 0 {
			fmt.Printf("⏳ Queued Jobs (%d): %s\n", len(resp.QueuedJobs), strings.Join(resp.QueuedJobs, ", "))
		} else {
			fmt.Println("⏳ Queued Jobs: 0")
		}

		if len(resp.UpcomingJobs) > 0 {
			fmt.Println("\n📅 Upcoming Scheduled Jobs:")
			fmt.Println("-----------------------------------------------------")
			for i, job := range resp.UpcomingJobs {
				if i >= 15 {
					fmt.Printf("   ... and %d more\n", len(resp.UpcomingJobs)-15)
					break
				}
				fmt.Printf("   [%s] %s (Cron: %s)\n", job.NextRun, job.Host, job.Schedule)
			}
		}

		
		fmt.Println("\n💾 Backup Storage Statistics:")
		fmt.Println("-----------------------------------------------------")
		fmt.Printf("Total Archives: %d\n", resp.TotalBackups)
		if resp.DiskTotal > 0 {
			pct := float64(resp.DiskFree) / float64(resp.DiskTotal) * 100
			fmt.Printf("Storage Usage:  %s / %s (%.1f%% Free)\n", formatSize(resp.DiskUsed), formatSize(resp.DiskTotal), pct)
		} else {
			fmt.Println("Storage Usage:  Unknown (Cannot stat directory)")
		}
		
		fmt.Println()
	},
}


func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
