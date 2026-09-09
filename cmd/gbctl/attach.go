package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"crypto/tls"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gobackup/internal/grpc/pb"
	"gobackup/internal/tui"
)

var attachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Attach to the live daemon log stream without triggering a new backup",
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
		client := pb.NewBackupServiceClient(conn)

		ui := tui.NewTviewUI()

		go func() {
			ui.Log("📡 Attaching to live daemon log stream...")
			stream, err := client.WatchLogs(context.Background(), &pb.WatchRequest{})
			if err != nil {
				ui.Log("❌ Stream Error: %v", err)
				time.Sleep(2 * time.Second)
				ui.Stop()
				return
			}

			for {
				chunk, err := stream.Recv()
				if err == io.EOF {
					ui.Log("🏁 Log stream ended by server.")
					break
				}
				if err != nil {
					ui.Log("❌ Connection lost: %v", err)
					break
				}

				if chunk.IsSummary {
					ui.Summary(chunk.Text)
					ui.SetStatus(fmt.Sprintf("Running: %s", chunk.HostName), true)
					if strings.Contains(chunk.Text, "Backup job complete!") || strings.Contains(chunk.Text, "Backup fatally failed") {
						break
					}
				} else {
					ui.Log("[%s] %s", chunk.HostName, chunk.Text)
				}
			}
			
			ui.SetStatus("Disconnected", false)
			ui.Stop()
		}()

		if err := ui.Start(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
