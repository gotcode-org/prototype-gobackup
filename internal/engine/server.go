package engine

import (
	"strings"
	"time"
	"sort"
	"github.com/robfig/cron/v3"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/credentials"
	
	pb "gobackup/internal/grpc/pb"
)

// Server implements both the BackupService and AdminService gRPC interfaces.
type Server struct {
	cfg Config
	pb.UnimplementedBackupServiceServer
	pb.UnimplementedAdminServiceServer
	db        *DB
	scheduler *Scheduler
}

func NewServer(cfg Config, db *DB, scheduler *Scheduler) *Server {
	return &Server{cfg: cfg, db: db, scheduler: scheduler}
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

	// 1. Setup TCP Server (Requires TLS & Interceptors)
	var tcpOpts []grpc.ServerOption
	tcpOpts = append(tcpOpts, grpc.UnaryInterceptor(AuthInterceptor(s.db)))
	tcpOpts = append(tcpOpts, grpc.StreamInterceptor(StreamAuthInterceptor(s.db)))
	if creds != nil {
		tcpOpts = append(tcpOpts, grpc.Creds(creds))
	}
	tcpServer := grpc.NewServer(tcpOpts...)
	pb.RegisterBackupServiceServer(tcpServer, s)
	pb.RegisterAdminServiceServer(tcpServer, s)

	// 2. Setup Unix Socket Server (NO TLS, Plain-text local God-Mode)
	unixOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(AuthInterceptor(s.db)),
		grpc.StreamInterceptor(StreamAuthInterceptor(s.db)),
	}
	unixServer := grpc.NewServer(unixOpts...)
	pb.RegisterBackupServiceServer(unixServer, s)
	pb.RegisterAdminServiceServer(unixServer, s)
	
	log.Printf("🚀 GoBackup Daemon running on TCP %s (TLS) and Unix Socket %s (Plaintext)", tcpAddr, sockPath)
	
	go unixServer.Serve(unixLis)
	return tcpServer.Serve(tcpLis)
}

// --- BackupService Implementation ---

func (s *Server) StartBackup(ctx context.Context, req *pb.BackupRequest) (*pb.BackupResponse, error) {
	log.Printf("Received StartBackup request for target: %s", req.Target)
	
	if req.Target == "" || req.Target == "all" {
		go func() {
			daemonUI := &DaemonLogger{hostName: "Global"}
			RunBackups(s.cfg, daemonUI)
		}()
		return &pb.BackupResponse{
			Success: true,
			Message: "Global backup initiated for all hosts",
			JobId:   "job-all",
		}, nil
	}

	var targetHost *HostConfig
	for _, h := range s.cfg.Hosts {
		if h.Name == req.Target {
			targetHost = &h
			break
		}
	}
	
	if targetHost == nil {
		return nil, status.Errorf(codes.NotFound, "host %s not found in configuration", req.Target)
	}

	// Kick off the backup asynchronously in the background
	go func(h HostConfig) {
		daemonUI := &DaemonLogger{hostName: h.Name}
		RunSingleBackup(s.cfg, h, daemonUI)
		daemonUI.SetStatus("Backup Complete!", false)
		daemonUI.Summary("🎉 Backup job complete!")
	}(*targetHost)

	return &pb.BackupResponse{
		Success: true,
		Message: fmt.Sprintf("Backup initiated for %s", req.Target),
		JobId:   "job-" + req.Target,
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

	if err := WriteHostConfig(s.cfg.ConfDir, host); err != nil {
		return nil, fmt.Errorf("failed to write YAML: %v", err)
	}

	if err := GitOpsSync(s.cfg.ConfDir, "chore: add backup host " + req.Name); err != nil {
		log.Printf("GitOps sync failed (ignoring for now): %v", err)
	}

	// Hot reload the scheduler and update memory!
	if s.scheduler != nil {
		s.scheduler.Stop()
		
		// Safely find the new host or update the existing one in memory
		found := false
		for i, h := range s.cfg.Hosts {
			if h.Name == host.Name {
				s.cfg.Hosts[i] = host
				found = true
				break
			}
		}
		if !found {
			s.cfg.Hosts = append(s.cfg.Hosts, host)
		}

		// Reboot scheduler with updated config
		s.scheduler = NewScheduler(s.cfg)
		s.scheduler.Start()
	}

	return &pb.AddHostResponse{
		Success: true,
		Message: "Host added and GitOps sync triggered successfully",
	}, nil
}

func (s *Server) ListHosts(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	var resp pb.ListResponse
	for _, h := range s.cfg.Hosts {
		resp.Hosts = append(resp.Hosts, &pb.HostInfo{
			Name:           h.Name,
			Address:        h.Address,
			Schedule:       h.Schedule,
			RetentionCount: int32(h.RetentionCount),
		})
	}
	return &resp, nil
}

func (s *Server) PruneBackups(ctx context.Context, req *pb.PruneRequest) (*pb.PruneResponse, error) {
	log.Println("Manual prune requested via gRPC")
	ui := &DaemonLogger{hostName: "prune"}
	CleanupOldBackups(s.cfg.BackupDir, s.cfg.Hosts, ui)
	return &pb.PruneResponse{
		Success: true,
		Message: "Prune complete",
	}, nil
}
func (s *Server) ListBackups(ctx context.Context, req *pb.ListBackupsRequest) (*pb.ListBackupsResponse, error) {
	var resp pb.ListBackupsResponse
	
	entries, err := os.ReadDir(s.cfg.BackupDir)
	if err != nil {
		return &resp, nil
	}

	for _, f := range entries {
		if f.IsDir() { continue }
		
		if strings.HasSuffix(f.Name(), ".tar.gz") {
			// Filename format: hostname_timestamp.tar.gz
			parts := strings.Split(f.Name(), "_")
			hostName := parts[0]

			if req.Target != "" && hostName != req.Target {
				continue
			}

			info, err := f.Info()
			if err != nil { continue }
			
			resp.Archives = append(resp.Archives, &pb.BackupArchive{
				Host:     hostName,
				Filename: f.Name(),
				Size:     info.Size(),
				Modified: info.ModTime().Format("2006-01-02 15:04:05"),
			})
		}
	}
	return &resp, nil
}

func (s *Server) GetStatus(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	StateMutex.Lock()
	defer StateMutex.Unlock()

	var upcoming []*pb.UpcomingJob
	now := time.Now()

	for _, h := range s.cfg.Hosts {
		if h.Schedule == "" { continue }
		
		schedule, err := cron.ParseStandard(h.Schedule)
		if err == nil {
			nextRun := schedule.Next(now)
			upcoming = append(upcoming, &pb.UpcomingJob{
				Host:     h.Name,
				Schedule: h.Schedule,
				NextRun:  nextRun.Format("2006-01-02 15:04:05"),
				NextUnix: nextRun.Unix(),
			})
		}
	}

	// Sort upcoming jobs chronologically
	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].NextUnix < upcoming[j].NextUnix
	})

	return &pb.StatusResponse{
		Online:       true,
		ActiveJob:    ActiveJob,
		QueuedJobs:   QueuedJobs,
		UpcomingJobs: upcoming,
	}, nil
}
