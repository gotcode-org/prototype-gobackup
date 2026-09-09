package engine

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	
	pb "gobackup/internal/grpc/pb"
)

// Server implements both the BackupService and AdminService gRPC interfaces.
type Server struct {
	pb.UnimplementedBackupServiceServer
	pb.UnimplementedAdminServiceServer
	db        *DB
	scheduler *Scheduler
}

func NewServer(db *DB, scheduler *Scheduler) *Server {
	return &Server{db: db, scheduler: scheduler}
}

// Start listens on the given TCP port and a local Unix socket, serving gRPC requests
func (s *Server) Start(port int, creds credentials.TransportCredentials) error {
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

	// Apply Interceptors and TLS
	var opts []grpc.ServerOption
	opts = append(opts, grpc.UnaryInterceptor(AuthInterceptor(s.db)))
	opts = append(opts, grpc.StreamInterceptor(StreamAuthInterceptor(s.db)))
	
	// Only apply TLS to the TCP listener if credentials are provided
	if creds != nil {
		opts = append(opts, grpc.Creds(creds))
	}

	grpcServer := grpc.NewServer(opts...)
	
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
	log.Printf("Client connected to log stream for job: %s", req.JobId)
	
	stream.Send(&pb.LogChunk{
		Text:      "🔗 Connected to GoBackup Daemon Log Stream...",
		IsSummary: false,
		HostName:  "daemon",
	})

	// Subscribe to the global log broadcaster
	ch := GlobalLogBroker.Subscribe()
	defer GlobalLogBroker.Unsubscribe(ch)

	for chunk := range ch {
		if err := stream.Send(chunk); err != nil {
			log.Printf("Client disconnected from log stream")
			return err
		}
	}
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

func (s *Server) AddHost(ctx context.Context, req *pb.AddHostRequest) (*pb.AddHostResponse, error) {
	log.Printf("Received AddHost request for %s", req.Name)

	host := HostConfig{
		Name:           req.Name,
		Group:          req.Group,
		Address:        req.Address,
		Port:           int(req.Port),
		UseSudo:        req.UseSudo,
		RetentionCount: int(req.RetentionCount),
		Paths:          req.Paths,
		Schedule:       req.Schedule,
	}

	if err := WriteHostConfig(host); err != nil {
		return nil, fmt.Errorf("failed to write YAML: %v", err)
	}

	if err := GitOpsSync("conf.d", "chore: add backup host " + req.Name); err != nil {
		log.Printf("GitOps sync failed (ignoring for now): %v", err)
	}

	// Hot reload the scheduler!
	if s.scheduler != nil {
		s.scheduler.Stop()
		cfg := LoadConfig("config.yaml")
		s.scheduler = NewScheduler(cfg)
		s.scheduler.Start()
	}

	return &pb.AddHostResponse{
		Success: true,
		Message: "Host added and GitOps sync triggered successfully",
	}, nil
}
