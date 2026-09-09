package engine

import (
	"log"
	"os"
	"strings"
	"gopkg.in/yaml.v3"
)

type Config struct {
	BackupDir  string       `yaml:"backup_dir"`
	WebhookURL string       `yaml:"webhook_url"`
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
	data, err := os.ReadFile(basePath)
	if err != nil {
		log.Fatalf("❌ Failed to read base config %s: %v", basePath, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("❌ Failed to parse base config %s: %v", basePath, err)
	}

	// Make sure conf.d exists
	confDir := "conf.d"
	os.MkdirAll(confDir, 0755)

	// Scan for individual host files
	files, err := os.ReadDir(confDir)
	if err == nil {
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".yaml") || strings.HasSuffix(f.Name(), ".yml") {
				hostData, err := os.ReadFile(confDir + "/" + f.Name())
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
func WriteHostConfig(host HostConfig) error {
	confDir := "conf.d"
	os.MkdirAll(confDir, 0755)

	data, err := yaml.Marshal(host)
	if err != nil {
		return err
	}

	filename := confDir + "/" + host.Name + ".yaml"
	return os.WriteFile(filename, data, 0644)
}
