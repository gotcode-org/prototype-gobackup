package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

type ClientConfig struct {
	ServerAddress string `json:"server_address"`
	Token         string `json:"token"`
}

func getClientConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gbctl.json" // fallback
	}
	dir := filepath.Join(home, ".config", "gobackup")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "auth.json")
}

func LoadClientConfig() ClientConfig {
	var cfg ClientConfig
	cfg.ServerAddress = "localhost:50051" // Default
	
	path := getClientConfigPath()
	b, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(b, &cfg)
	}
	return cfg
}

func SaveClientConfig(cfg ClientConfig) error {
	path := getClientConfigPath()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

// tokenAuth implements grpc.PerRPCCredentials
type tokenAuth struct {
	token string
}
func (t tokenAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + t.token}, nil
}
func (t tokenAuth) RequireTransportSecurity() bool { return false } // allow insecure for now
