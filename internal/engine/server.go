package engine

import (
	"path/filepath"
	"strings"
	"time"
	"syscall"
	"sort"
	"github.com/robfig/cron/v3"
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
	count := 0
	for _, job := range s.cfg.Jobs {
		if req.TargetServer != "" && job.Server != req.TargetServer { continue }
		if req.TargetJob != "" && job.Name != req.TargetJob { continue }
		count++
		go func(j JobConfig) {
			daemonUI := &DaemonLogger{hostName: j.Name}
			RunSingleBackup(s.cfg, j, daemonUI)
		}(job)
	}
	if count == 0 { return nil, fmt.Errorf("no matching jobs found") }
	return &pb.BackupResponse{Success: true, Message: fmt.Sprintf("Started %d jobs in the background", count)}, nil
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

func (s *Server) AddServer(ctx context.Context, req *pb.AddServerRequest) (*pb.GenericResponse, error) {
	srv := ServerConfig{
		Name:    req.Name,
		Group:   req.Group,
		Address: req.Address,
		Port:    int(req.Port),
		UseSudo: req.UseSudo,
	}
	
	found := false
	for i, existing := range s.cfg.Servers {
		if existing.Name == srv.Name {
			s.cfg.Servers[i] = srv
			found = true
			break
		}
	}
	if !found {
		s.cfg.Servers = append(s.cfg.Servers, srv)
	}

	if err := WriteServerConfig(s.cfg.ConfDir, srv); err != nil {
		return nil, fmt.Errorf("failed to save server config: %v", err)
	}
	return &pb.GenericResponse{Success: true, Message: "Server added successfully"}, nil
}

func (s *Server) AddJob(ctx context.Context, req *pb.AddJobRequest) (*pb.GenericResponse, error) {
	job := JobConfig{
		Name:           req.Name,
		Server:         req.Server,
		Schedule:       req.Schedule,
		RetentionCount: int(req.RetentionCount),
		Incremental:    req.Incremental,
		FullInterval:   int(req.FullInterval),
		Paths:          req.Paths,
		DockerVolumes:  req.DockerVolumes,
		PreBackup:      JobPreBackup{PauseContainers: req.PauseContainers},
	}
	
	found := false
	for i, existing := range s.cfg.Jobs {
		if existing.Name == job.Name && existing.Server == job.Server {
			s.cfg.Jobs[i] = job
			found = true
			break
		}
	}
	if !found {
		s.cfg.Jobs = append(s.cfg.Jobs, job)
	}

	if err := WriteJobConfig(s.cfg.ConfDir, job); err != nil {
		return nil, fmt.Errorf("failed to save job config: %v", err)
	}
	
	if s.scheduler != nil {
		s.scheduler.Stop()
		s.scheduler = NewScheduler(s.cfg)
		s.scheduler.Start()
	}
	return &pb.GenericResponse{Success: true, Message: "Job added and scheduler reloaded"}, nil
}

func (s *Server) ListServers(ctx context.Context, req *pb.ListRequest) (*pb.ListServerResponse, error) {
	var resp pb.ListServerResponse
	for _, srv := range s.cfg.Servers {
		resp.Servers = append(resp.Servers, &pb.ServerInfo{
			Name:    srv.Name,
			Group:   srv.Group,
			Address: srv.Address,
			Port:    int32(srv.Port),
			UseSudo: srv.UseSudo,
		})
	}
	return &resp, nil
}

func (s *Server) ListJobs(ctx context.Context, req *pb.ListRequest) (*pb.ListJobResponse, error) {
	var resp pb.ListJobResponse
	for _, job := range s.cfg.Jobs {
		resp.Jobs = append(resp.Jobs, &pb.JobInfo{
			Name:           job.Name,
			Server:         job.Server,
			Schedule:       job.Schedule,
			RetentionCount: int32(job.RetentionCount),
		})
	}
	return &resp, nil
}

func (s *Server) PruneBackups(ctx context.Context, req *pb.PruneRequest) (*pb.PruneResponse, error) {
	log.Println("Manual prune requested via gRPC")
	ui := &DaemonLogger{hostName: "prune"}
	CleanupOldBackups(s.cfg.BackupDir, s.cfg.Jobs, ui)
	return &pb.PruneResponse{
		Success: true,
		Message: "Prune complete",
	}, nil
}
func (s *Server) ListBackups(ctx context.Context, req *pb.ListBackupsRequest) (*pb.ListBackupsResponse, error) {
	var resp pb.ListBackupsResponse
	
	dirs := []struct{ Path, Type string }{
		{s.cfg.BackupDir, "SYSTEM"},
		{filepath.Join(s.cfg.BackupDir, "docker-volume"), "DOCKER"},
	}

	for _, d := range dirs {
		entries, err := os.ReadDir(d.Path)
		if err != nil {
			continue
		}

		for _, f := range entries {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".tar.gz") {
				continue
			}
			parts := strings.Split(f.Name(), "_")
			if len(parts) < 3 { continue }
			serverName := parts[0]
			jobName := parts[1]

			if req.Target != "" && (serverName != req.Target && jobName != req.Target) {
				continue
			}

			info, err := f.Info()
			if err != nil { continue }
			
			resp.Archives = append(resp.Archives, &pb.BackupArchive{
				Server:   serverName,
				Job:      jobName,
				Filename: f.Name(),
				Size:     info.Size(),
				Modified: info.ModTime().Format("2006-01-02 15:04:05"),
				Type:     d.Type,
			})
		}
	}
	
	// Sort by mod time descending (newest first) inside the RPC if needed, 
	// but currently the slice is not sorted here. We'll let the client sort or sort here:
	sort.Slice(resp.Archives, func(i, j int) bool {
		return resp.Archives[i].Modified > resp.Archives[j].Modified
	})
	
	return &resp, nil
}

func (s *Server) GetStatus(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	StateMutex.Lock()
	defer StateMutex.Unlock()

	var upcoming []*pb.UpcomingJob
	now := time.Now()

	for _, h := range s.cfg.Jobs {
		if h.Schedule == "" { continue }
		
		schedule, err := cron.ParseStandard(h.Schedule)
		if err == nil {
			nextRun := schedule.Next(now)
			upcoming = append(upcoming, &pb.UpcomingJob{
				Job:     h.Name,
                Server:  h.Server,
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

	totalDisk, freeDisk, usedDisk := getDiskInfo(s.cfg.BackupDir)
	
	var totalBackups int32 = 0
	if files, err := os.ReadDir(s.cfg.BackupDir); err == nil {
		for _, entry := range files {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tar.gz") {
				totalBackups++
			}
		}
	}

	return &pb.StatusResponse{
		Online:       true,
		ActiveJob:    ActiveJob,
		QueuedJobs:   QueuedJobs,
		UpcomingJobs: upcoming,
		DiskTotal:    totalDisk,
		DiskUsed:     usedDisk,
		DiskFree:     freeDisk,
		TotalBackups: totalBackups,
	}, nil
}

func getDiskInfo(path string) (total, free, used int64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0
	}
	total = int64(stat.Blocks) * int64(stat.Bsize)
	free = int64(stat.Bavail) * int64(stat.Bsize)
	used = total - free
	return total, free, used
}

func (s *Server) RemoveServer(ctx context.Context, req *pb.RemoveServerRequest) (*pb.GenericResponse, error) {
	found := false
	for i, srv := range s.cfg.Servers {
		if srv.Name == req.Name {
			s.cfg.Servers = append(s.cfg.Servers[:i], s.cfg.Servers[i+1:]...)
			found = true
			break
		}
	}
	if !found { return nil, fmt.Errorf("server %s not found", req.Name) }
	os.Remove(filepath.Join(s.cfg.ConfDir, "servers", req.Name+".yaml"))
	return &pb.GenericResponse{Success: true, Message: "Server removed"}, nil
}

func (s *Server) RemoveJob(ctx context.Context, req *pb.RemoveJobRequest) (*pb.GenericResponse, error) {
	found := false
	for i, job := range s.cfg.Jobs {
		if job.Name == req.Name && job.Server == req.Server {
			s.cfg.Jobs = append(s.cfg.Jobs[:i], s.cfg.Jobs[i+1:]...)
			found = true
			break
		}
	}
	if !found { return nil, fmt.Errorf("job %s/%s not found", req.Server, req.Name) }
	os.Remove(filepath.Join(s.cfg.ConfDir, "jobs", req.Server, req.Name+".yaml"))
	if s.scheduler != nil {
		s.scheduler.Stop()
		s.scheduler = NewScheduler(s.cfg)
		s.scheduler.Start()
	}
	return &pb.GenericResponse{Success: true, Message: "Job removed"}, nil
}

func (s *Server) RemoveBackup(ctx context.Context, req *pb.RemoveBackupRequest) (*pb.GenericResponse, error) {
	// Try root backup dir
	rootPath := filepath.Join(s.cfg.BackupDir, req.Filename)
	if err := os.Remove(rootPath); err == nil {
		return &pb.GenericResponse{Success: true, Message: "Backup removed."}, nil
	}
	// Try docker-volume dir
	dockerPath := filepath.Join(s.cfg.BackupDir, "docker-volume", req.Filename)
	if err := os.Remove(dockerPath); err == nil {
		return &pb.GenericResponse{Success: true, Message: "Backup removed."}, nil
	}
	return nil, fmt.Errorf("backup file not found")
}
