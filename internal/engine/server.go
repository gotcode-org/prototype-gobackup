package engine

import (
	"context"
	"fmt"
	"log"
	"net"
	
	"google.golang.org/grpc"
	
	pb "gobackup/internal/grpc/pb"
)

// Server implements both the BackupService and AdminService gRPC interfaces.
type Server struct {
	pb.UnimplementedBackupServiceServer
	pb.UnimplementedAdminServiceServer
}

func NewServer() *Server {
	return &Server{}
}

// Start listens on the given port and serves gRPC requests
func (s *Server) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	
	grpcServer := grpc.NewServer()
	
	// Register both services
	pb.RegisterBackupServiceServer(grpcServer, s)
	pb.RegisterAdminServiceServer(grpcServer, s)
	
	log.Printf("🚀 GoBackup Daemon running on %s", addr)
	return grpcServer.Serve(lis)
}

// --- BackupService Implementation ---

func (s *Server) StartBackup(ctx context.Context, req *pb.BackupRequest) (*pb.BackupResponse, error) {
	log.Printf("Received StartBackup request for target: %s", req.Target)
	return &pb.BackupResponse{
		Success: true,
		Message: fmt.Sprintf("Backup initiated for %s", req.Target),
		JobId:   "job-1234", // Stub
	}, nil
}

func (s *Server) WatchLogs(req *pb.WatchRequest, stream pb.BackupService_WatchLogsServer) error {
	log.Printf("Client watching logs for job: %s", req.JobId)
	
	// Stub: Send a fake log message
	stream.Send(&pb.LogChunk{
		Text:      "Daemon connected, awaiting logs...",
		IsSummary: false,
		HostName:  "daemon",
	})
	
	return nil
}

func (s *Server) GetStatus(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	return &pb.StatusResponse{
		ActiveJobs: 0,
		Uptime:     "Just started",
	}, nil
}

// --- AdminService Implementation ---

func (s *Server) GenerateToken(ctx context.Context, req *pb.GenerateTokenRequest) (*pb.GenerateTokenResponse, error) {
	log.Printf("Received GenerateToken request for user: %s (Role: %s)", req.Username, req.Role)
	return &pb.GenerateTokenResponse{
		Token: "stub-token-abc-123",
	}, nil
}
