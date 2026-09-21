package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gobackup/internal/grpc/pb"
)


var (
	filterSystem bool
	filterDocker bool
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
		resp, err := client.ListServers(context.Background(), &pb.ListRequest{})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }

		if len(resp.Servers) == 0 {
			fmt.Println("No configured backup hosts found.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
		fmt.Fprintln(w, "SERVER\tGROUP\tADDRESS\tPORT\tSUDO")
		fmt.Fprintln(w, "------\t-----\t-------\t----\t----")
		for _, h := range resp.Servers {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%v\n", h.Name, h.Group, h.Address, h.Port, h.UseSudo)
		}
		w.Flush()
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

		var filteredArchives []*pb.BackupArchive
		for _, a := range resp.Archives {
			if filterSystem && !filterDocker && a.Type != "SYSTEM" {
				continue
			}
			if filterDocker && !filterSystem && a.Type != "DOCKER" {
				continue
			}
			filteredArchives = append(filteredArchives, a)
		}

		if len(filteredArchives) == 0 {
			fmt.Println("No backups found matching criteria.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
		fmt.Fprintln(w, "HOST	TYPE	ARCHIVE	SIZE	MODIFIED")
		fmt.Fprintln(w, "----	----	-------	----	--------")
		for _, a := range filteredArchives {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", a.Job, a.Type, a.Filename, formatSize(a.Size), a.Modified)
		}
		w.Flush()
	},
}

func init() {
	listCmd.AddCommand(listHostsCmd)
	listBackupsCmd.Flags().BoolVar(&filterSystem, "system", false, "Filter to show only SYSTEM backups")
	listBackupsCmd.Flags().BoolVar(&filterDocker, "docker", false, "Filter to show only DOCKER backups")
	listCmd.AddCommand(listBackupsCmd)
	rootCmd.AddCommand(listCmd)
}

var listJobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "List all configured backup jobs",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
			grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token}),
		}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewBackupServiceClient(conn)
		resp, err := client.ListJobs(context.Background(), &pb.ListRequest{})
		if err != nil { log.Fatalf("❌ Failed to list jobs: %v", err) }
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