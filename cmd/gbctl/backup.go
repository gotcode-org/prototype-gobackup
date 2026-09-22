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
	"strings"
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
	Use:   "list [host] [job] | [exact_full_backup_filename]",
	Short: "List backup archives",
	Args:  cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		req := &pb.ListBackupsRequest{}
		var exactChain string
		if len(args) == 1 {
			if strings.HasSuffix(args[0], ".tar.gz") && strings.Contains(args[0], "_FULL_") {
				parts := strings.Split(args[0], "_FULL_")
				req.Target = parts[0] + "_"
				exactChain = args[0]
			} else {
				req.Target = args[0]
			}
		} else if len(args) == 2 {
			req.Target = args[0] + "_" + args[1] + "_"
		}
		
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewBackupServiceClient(conn)
		resp, err := client.ListBackups(context.Background(), req)
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		
		if exactChain != "" {
			var chainFiltered []*pb.BackupArchive
			inChain := false
			for _, a := range resp.Archives {
				if a.Filename == exactChain {
					inChain = true
					chainFiltered = append(chainFiltered, a)
					continue
				}
				if inChain {
					if a.ArchiveType == "FULL" { break }
					chainFiltered = append(chainFiltered, a)
				}
			}
			resp.Archives = chainFiltered
		}

		var filtered []*pb.BackupArchive
		for _, a := range resp.Archives {
			if filterSystem && !filterDocker && a.Type != "SYSTEM" { continue }
			if filterDocker && !filterSystem && a.Type != "DOCKER" { continue }
			filtered = append(filtered, a)
		}
		if len(filtered) == 0 { fmt.Println("No backups found."); return }
		
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
		fmt.Fprintln(w, "SERVER\tJOB\tARCHIVE TYPE\tARCHIVE\tSIZE\tMODIFIED")
		fmt.Fprintln(w, "------\t---\t------------\t-------\t----\t--------")
		for _, a := range filtered {
			typeStr := "[" + a.ArchiveType + "]"
			if a.ArchiveType == "INC" {
				typeStr = "  └- " + typeStr
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", a.Server, a.Job, typeStr, a.Filename, formatSize(a.Size), a.Modified)
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




var restoreDest string

var backupRestoreCmd = &cobra.Command{
	Use:   "restore [filename]",
	Short: "Restore a backup archive to a remote directory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewAdminServiceClient(conn)
		
		req := &pb.RestoreBackupRequest{Filename: args[0], TargetDir: restoreDest}
		stream, err := client.RestoreBackup(context.Background(), req)
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		
		for {
			chunk, err := stream.Recv()
			if err != nil { break }
			fmt.Print(chunk.Content)
			if chunk.Status == "DONE" || chunk.Status == "ERROR" {
				break
			}
		}
	},
}

func init() {
	backupListCmd.Flags().BoolVar(&filterSystem, "system", false, "Filter to show only SYSTEM backups")
	backupListCmd.Flags().BoolVar(&filterDocker, "docker", false, "Filter to show only DOCKER backups")
	
	backupCmd.AddCommand(backupListCmd, backupRmCmd, backupRestoreCmd)
	backupRestoreCmd.Flags().StringVar(&restoreDest, "dest", "/home/backup/RESTORE", "Target directory to restore the backup into")
	rootCmd.AddCommand(backupCmd)
}
