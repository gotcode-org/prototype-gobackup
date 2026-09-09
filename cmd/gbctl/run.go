package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"crypto/tls"

	pb "gobackup/internal/grpc/pb"
	"gobackup/internal/tui"
)

// tokenAuth implements grpc.PerRPCCredentials
type tokenAuth struct {
	token string
}
func (t tokenAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + t.token}, nil
}
func (t tokenAuth) RequireTransportSecurity() bool { return false } // allow insecure for now

var runCmd = &cobra.Command{
	Use:   "run [server/group]",
	Short: "Trigger a backup on the daemon and stream logs",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetArg := ""
		if len(args) == 1 {
			targetArg = args[0]
		}

		cfg := LoadClientConfig()
		if cfg.Token == "" {
			log.Fatalf("❌ Not authenticated. Please run: gbctl login <TOKEN>")
		}

		// Connect to gRPC server
		opts := []grpc.DialOption{
			// Connect using TLS. Skip strict cert validation to easily support auto-generated self-signed certs.
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
			grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token}),
		}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil {
			log.Fatalf("❌ Failed to connect to daemon at %s: %v", cfg.ServerAddress, err)
		}
		defer conn.Close()
		client := pb.NewBackupServiceClient(conn)

		// Start TUI
		ui := tui.NewTviewUI()

		go func() {
			// Trigger backup unconditionally
			targetReq := targetArg
			if targetReq == "" {
				targetReq = "all"
			}
			ui.Log("📡 Sending StartBackup request for: %s", targetReq)
			resp, err := client.StartBackup(context.Background(), &pb.BackupRequest{Target: targetReq})
			if err != nil {
				ui.Log("❌ RPC Error: %v", err)
				time.Sleep(2 * time.Second)
				ui.Stop()
				return
			}
			ui.Log("✅ Server accepted job: %s", resp.Message)

			// Watch Logs
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
					if strings.Contains(chunk.Text, "Backup job complete!") {
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
	rootCmd.AddCommand(runCmd)
}
