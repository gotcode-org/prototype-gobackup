package engine

import (
		"os"
	"strings"
	"gopkg.in/yaml.v3"
)

type Config struct {
	BackupDir  string       `yaml:"backup_dir"`
	WebhookURL string       `yaml:"webhook_url"`
	ConfDir    string       `yaml:"conf_dir"`
	DBPath     string       `yaml:"db_path"`
	TLSCert    string       `yaml:"tls_cert"`
	TLSKey     string       `yaml:"tls_key"`
	Hosts      []HostConfig `yaml:"hosts"`
}

type HostConfig struct {
	Name           string   `yaml:"name"`
	Group          string   `yaml:"group"`
	Address        string   `yaml:"address"`
	Port           int      `yaml:"port"`
	UseSudo        bool     `yaml:"use_sudo"`
	Schedule       string   `yaml:"schedule"`
	RetentionCount int      `yaml:"retention_count"`
	Paths          []string `yaml:"paths"`
}

// LoadConfig parses a base config.yaml, and then recursively reads all individual host YAMLs inside a conf.d/ directory.
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

	os.MkdirAll(cfg.ConfDir, 0755)

	// Scan for individual host files
	files, err := os.ReadDir(cfg.ConfDir)
	if err == nil {
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".yaml") || strings.HasSuffix(f.Name(), ".yml") {
				hostData, err := os.ReadFile(cfg.ConfDir + "/" + f.Name())
				if err == nil {
					var host HostConfig
					if yaml.Unmarshal(hostData, &host) == nil {
						cfg.Hosts = append(cfg.Hosts, host)
					}
				}
			}
		}
	}

	return cfg
}

// WriteHostConfig dynamically generates a YAML file for a single host in the conf.d/ directory
func WriteHostConfig(confDir string, host HostConfig) error {

	os.MkdirAll(confDir, 0755)

	data, err := yaml.Marshal(host)
	if err != nil {
		return err
	}

	filename := confDir + "/" + host.Name + ".yaml"
	return os.WriteFile(filename, data, 0644)
}
