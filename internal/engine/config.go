package engine

import (
	"log"
	"os"
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
	RetentionCount int      `yaml:"retention_count"`
	Paths          []string `yaml:"paths"`
}

func LoadConfig(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("❌ Failed to read config %s: %v", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("❌ Failed to parse config %s: %v", path, err)
	}
	return cfg
}
