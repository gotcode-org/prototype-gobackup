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

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Manage backup archives",
}

var backupListCmd = &cobra.Command{
	Use:   "list [optional job filter]",
	Short: "List backup archives",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		req := &pb.ListBackupsRequest{}
		if len(args) > 0 { req.Target = args[0] }
		
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewBackupServiceClient(conn)
		resp, err := client.ListBackups(context.Background(), req)
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		
		var filtered []*pb.BackupArchive
		for _, a := range resp.Archives {
			if filterSystem && !filterDocker && a.Type != "SYSTEM" { continue }
			if filterDocker && !filterSystem && a.Type != "DOCKER" { continue }
			filtered = append(filtered, a)
		}
		if len(filtered) == 0 { fmt.Println("No backups found."); return }
		
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
		fmt.Fprintln(w, "JOB\tTYPE\tARCHIVE\tSIZE\tMODIFIED")
		fmt.Fprintln(w, "---\t----\t-------\t----\t--------")
		for _, a := range filtered {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", a.Job, a.Server, a.Type, a.Filename, formatSize(a.Size), a.Modified)
		}
		w.Flush()
	},
}

var backupRmCmd = &cobra.Command{
	Use:   "rm [filename]",
	Short: "Remove a backup archive",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewAdminServiceClient(conn)
		resp, err := client.RemoveBackup(context.Background(), &pb.RemoveBackupRequest{Filename: args[0]})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		fmt.Printf("✅ %s\n", resp.Message)
	},
}



func init() {
	backupListCmd.Flags().BoolVar(&filterSystem, "system", false, "Filter to show only SYSTEM backups")
	backupListCmd.Flags().BoolVar(&filterDocker, "docker", false, "Filter to show only DOCKER backups")
	
	backupCmd.AddCommand(backupListCmd, backupRmCmd)
	rootCmd.AddCommand(backupCmd)
}
