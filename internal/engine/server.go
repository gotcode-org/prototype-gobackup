package engine

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	
	"google.golang.org/grpc"
	
	pb "gobackup/internal/grpc/pb"
)

// Server implements both the BackupService and AdminService gRPC interfaces.
type Server struct {
	pb.UnimplementedBackupServiceServer
	pb.UnimplementedAdminServiceServer
	db *DB
}

func NewServer(db *DB) *Server {
	return &Server{db: db}
}

// Start listens on the given TCP port and a local Unix socket, serving gRPC requests
func (s *Server) Start(port int) error {
	tcpAddr := fmt.Sprintf(":%d", port)
	tcpLis, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on TCP: %w", err)
	}
	
	sockPath := "/tmp/gobackupd.sock"
	_ = os.Remove(sockPath) // Clean up old socket if it exists
	unixLis, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("failed to listen on Unix Socket: %w", err)
	}

	// Apply Interceptors for SQLite Token Auth and God-Mode Bypass
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(AuthInterceptor(s.db)),
		grpc.StreamInterceptor(StreamAuthInterceptor(s.db)),
	)
	
	// Register both services
	pb.RegisterBackupServiceServer(grpcServer, s)
	pb.RegisterAdminServiceServer(grpcServer, s)
	
	log.Printf("🚀 GoBackup Daemon running on TCP %s and Unix Socket %s", tcpAddr, sockPath)
	
	go grpcServer.Serve(unixLis)
	return grpcServer.Serve(tcpLis)
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
	
	rawToken, err := s.db.CreateUser(req.Username, req.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &pb.GenerateTokenResponse{
		Token: rawToken,
	}, nil
}
