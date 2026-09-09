package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	pb "gobackup/internal/grpc/pb"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Admin commands (requires local root/sudo via Unix Socket God-Mode)",
}

var generateTokenCmd = &cobra.Command{
	Use:   "generate-token [username] [role]",
	Short: "Generate a new auth token (bypasses Auth via Unix Socket)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		username := args[0]
		role := args[1]

		// Dial the local Unix socket
		conn, err := grpc.Dial("/tmp/gobackupd.sock", grpc.WithInsecure(), grpc.WithContextDialer(
			func(ctx context.Context, addr string) (net.Conn, error) {
				return net.Dial("unix", addr)
			}))
		if err != nil {
			log.Fatalf("❌ Failed to connect to local daemon socket: %v", err)
		}
		defer conn.Close()

		client := pb.NewAdminServiceClient(conn)
		
		resp, err := client.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
			Username: username,
			Role:     role,
		})
		if err != nil {
			log.Fatalf("❌ Admin RPC Error: %v", err)
		}

		fmt.Printf("✅ Token generated for %s!\n", username)
		fmt.Printf("🔑 Token: %s\n", resp.Token)
		fmt.Printf("Run 'gbctl login %s' to authenticate this client.\n", resp.Token)
	},
}

func init() {
	adminCmd.AddCommand(generateTokenCmd)
	rootCmd.AddCommand(adminCmd)
}
