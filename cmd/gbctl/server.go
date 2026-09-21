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
	serverGroup string
	serverPort  int
	serverSudo  bool
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage servers",
}

var serverAddCmd = &cobra.Command{
	Use:   "add [name] [address]",
	Short: "Add a new server",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		req := &pb.AddServerRequest{Name: args[0], Address: args[1], Group: serverGroup, Port: int32(serverPort), UseSudo: serverSudo}
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewAdminServiceClient(conn)
		resp, err := client.AddServer(context.Background(), req)
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		fmt.Printf("✅ %s\n", resp.Message)
	},
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List servers",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewBackupServiceClient(conn)
		resp, err := client.ListServers(context.Background(), &pb.ListRequest{})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		if len(resp.Servers) == 0 { fmt.Println("No configured servers found."); return }
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
		fmt.Fprintln(w, "SERVER\tGROUP\tADDRESS\tPORT\tSUDO")
		fmt.Fprintln(w, "------\t-----\t-------\t----\t----")
		for _, s := range resp.Servers {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%v\n", s.Name, s.Group, s.Address, s.Port, s.UseSudo)
		}
		w.Flush()
	},
}

var serverRmCmd = &cobra.Command{
	Use:   "rm [name]",
	Short: "Remove a server",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := LoadClientConfig()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})), grpc.WithPerRPCCredentials(tokenAuth{token: cfg.Token})}
		conn, err := grpc.Dial(cfg.ServerAddress, opts...)
		if err != nil { log.Fatalf("❌ Failed to connect: %v", err) }
		defer conn.Close()
		client := pb.NewAdminServiceClient(conn)
		resp, err := client.RemoveServer(context.Background(), &pb.RemoveServerRequest{Name: args[0]})
		if err != nil { log.Fatalf("❌ RPC Error: %v", err) }
		fmt.Printf("✅ %s\n", resp.Message)
	},
}

func init() {
	serverAddCmd.Flags().StringVarP(&serverGroup, "group", "g", "default", "Server group name")
	serverAddCmd.Flags().IntVarP(&serverPort, "port", "p", 22, "SSH Port")
	serverAddCmd.Flags().BoolVar(&serverSudo, "sudo", false, "Use sudo for execution")
	
	serverCmd.AddCommand(serverAddCmd, serverListCmd, serverRmCmd)
	rootCmd.AddCommand(serverCmd)
}
