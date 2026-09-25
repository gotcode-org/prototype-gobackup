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

var showAll bool

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
			parts := strings.SplitN(resp.ActiveJob, "_", 2)
			if len(parts) == 2 {
				fmt.Printf("🔥 Active Job: %s / %s\n", parts[0], parts[1])
			} else {
				fmt.Printf("🔥 Active Job: %s\n", resp.ActiveJob)
			}
		} else {
			fmt.Println("💤 Active Job: None (Idle)")
		}

		if len(resp.QueuedJobs) > 0 {
			fmt.Printf("\n⏳ Queued Jobs (%d):\n", len(resp.QueuedJobs))
			qw := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
			fmt.Fprintln(qw, "QUEUE POS\tSERVER\tJOB")
			fmt.Fprintln(qw, "---------\t------\t---")
			for i, qj := range resp.QueuedJobs {
				parts := strings.SplitN(qj, "_", 2)
				srv := qj
				jobName := ""
				if len(parts) == 2 {
					srv = parts[0]
					jobName = parts[1]
				}
				fmt.Fprintf(qw, "%d\t%s\t%s\n", i+1, srv, jobName)
			}
			qw.Flush()
		} else {
			fmt.Println("\n⏳ Queued Jobs: 0")
		}

		if len(resp.UpcomingJobs) > 0 {
			fmt.Println("\n📅 Upcoming Scheduled Jobs:")
			w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
			fmt.Fprintln(w, "SERVER\tJOB\tNEXT RUN\tSCHEDULE")
			fmt.Fprintln(w, "------\t---\t--------\t--------")
			for i, job := range resp.UpcomingJobs {
				if i >= 15 && !showAll {
					break
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", job.Server, job.Job, job.NextRun, job.Schedule)
			}
			w.Flush()
			if len(resp.UpcomingJobs) > 15 && !showAll {
				fmt.Printf("... and %d more\n", len(resp.UpcomingJobs)-15)
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
	statusCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all upcoming jobs instead of truncating")
	rootCmd.AddCommand(statusCmd)
}
