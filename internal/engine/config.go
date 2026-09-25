package engine

import (
	"os"
	"path/filepath"
	"strings"
	"gopkg.in/yaml.v3"
)

type NotificationConfig struct {
	Type     string `yaml:"type"` // "discord", "email", "m365_graph", "webhook"
	URL      string `yaml:"url,omitempty"`
	SMTPHost string `yaml:"smtp_host,omitempty"`
	SMTPPort int    `yaml:"smtp_port,omitempty"`
	SMTPUser string `yaml:"smtp_user,omitempty"`
	SMTPPass string `yaml:"smtp_pass,omitempty"`
	To       string `yaml:"to,omitempty"`
	From     string `yaml:"from,omitempty"`
	TenantID string `yaml:"tenant_id,omitempty"`
	ClientID string `yaml:"client_id,omitempty"`
	Secret   string `yaml:"secret,omitempty"`
}

type Config struct {
	BackupDir     string               `yaml:"backup_dir"`
	ColdStoragePath string             `yaml:"cold_storage_path"`
	Notifications []NotificationConfig `yaml:"notifications"` // Replaced single WebhookURL
	ConfDir       string               `yaml:"conf_dir"`
	DBPath        string               `yaml:"db_path"`
	TLSCert       string               `yaml:"tls_cert"`
	TLSKey        string               `yaml:"tls_key"`
	Servers       []ServerConfig       `yaml:"servers"`
	Jobs          []JobConfig          `yaml:"jobs"`
}

type ServerConfig struct {
	Name    string `yaml:"name"`
	Group   string `yaml:"group"`
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
	UseSudo bool   `yaml:"use_sudo"`
}

type JobPreBackup struct {
	PauseContainers []string `yaml:"pause_containers"`
}

type ColdStorageConfig struct {
	Path           string `yaml:"path"`
	RetentionCount int    `yaml:"retention_count"`
}

type JobConfig struct {
	Name           string       `yaml:"name"`
	Server         string       `yaml:"server"`
	Schedule       string       `yaml:"schedule"`
	RetentionCount int          `yaml:"retention_count"`
	Incremental    bool         `yaml:"incremental"`
	FullInterval   int          `yaml:"full_interval"`
	Paths          []string     `yaml:"paths"`
	DockerVolumes  []string     `yaml:"docker_volumes"`
	HotStoragePath string       `yaml:"hot_storage_path"`
	PreBackup      JobPreBackup `yaml:"pre_backup"`
	ColdStorage    ColdStorageConfig `yaml:"cold_storage"`
}

func LoadConfig(basePath string) Config {
	var cfg Config
	data, err := os.ReadFile(basePath)
	if err == nil {
		yaml.Unmarshal(data, &cfg)
	}

	if cfg.ConfDir == "" { cfg.ConfDir = "/etc/gobackup/conf.d" }
	if cfg.DBPath == "" { cfg.DBPath = "/etc/gobackup/gobackup.db" }
	if cfg.TLSCert == "" { cfg.TLSCert = "/etc/gobackup/server.crt" }
	if cfg.TLSKey == "" { cfg.TLSKey = "/etc/gobackup/server.key" }
	if cfg.BackupDir == "" { cfg.BackupDir = "/var/lib/gobackup/backups" }

	serversDir := filepath.Join(cfg.ConfDir, "servers")
	jobsDir := filepath.Join(cfg.ConfDir, "jobs")
	os.MkdirAll(serversDir, 0755)
	os.MkdirAll(jobsDir, 0755)

	files, err := os.ReadDir(serversDir)
	if err == nil {
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".yaml") || strings.HasSuffix(f.Name(), ".yml") {
				data, err := os.ReadFile(filepath.Join(serversDir, f.Name()))
				if err == nil {
					var server ServerConfig
					if yaml.Unmarshal(data, &server) == nil {
						cfg.Servers = append(cfg.Servers, server)
					}
				}
			}
		}
	}

	serverDirs, err := os.ReadDir(jobsDir)
	if err == nil {
		for _, sd := range serverDirs {
			if sd.IsDir() {
				jobFiles, err := os.ReadDir(filepath.Join(jobsDir, sd.Name()))
				if err == nil {
					for _, f := range jobFiles {
						if strings.HasSuffix(f.Name(), ".yaml") || strings.HasSuffix(f.Name(), ".yml") {
							data, err := os.ReadFile(filepath.Join(jobsDir, sd.Name(), f.Name()))
							if err == nil {
								var job JobConfig
								if yaml.Unmarshal(data, &job) == nil {
									cfg.Jobs = append(cfg.Jobs, job)
								}
							}
						}
					}
				}
			}
		}
	}

	return cfg
}

func WriteServerConfig(confDir string, server ServerConfig) error {
	dir := filepath.Join(confDir, "servers")
	os.MkdirAll(dir, 0755)
	data, err := yaml.Marshal(server)
	if err != nil { return err }
	return os.WriteFile(filepath.Join(dir, server.Name+".yaml"), data, 0644)
}

func WriteJobConfig(confDir string, job JobConfig) error {
	dir := filepath.Join(confDir, "jobs", job.Server)
	os.MkdirAll(dir, 0755)
	data, err := yaml.Marshal(job)
	if err != nil { return err }
	return os.WriteFile(filepath.Join(dir, job.Name+".yaml"), data, 0644)
}
