# GoBackup v2: Client/Server Architecture Plan

This document outlines the roadmap for converting the GoBackup monolithic script into a highly concurrent, multi-user gRPC Client/Server Daemon.

## Phase 0: Standard Go Project Restructuring
- [ ] **Directory Layout:** Reorganize the monolithic codebase into the standard Go project layout.
  - `cmd/gobackupd/`: Entrypoint for the background daemon (Server).
  - `cmd/gobackup/` (or `gbctl/`): Entrypoint for the thin CLI/TUI client.
  - `internal/engine/`: Core business logic (cron scheduling, auth, tar/ssh execution).
  - `internal/tui/`: Bubbletea/Tview presentation logic.
  - `internal/grpc/`: Protobuf generated code and client/server interceptors.
- [ ] **Cobra CLI Integration:** Implement `spf13/cobra` for robust command routing.
  - Ensure the `cmd/` packages remain strictly for command registration and flag parsing.
  - Offload all actual execution logic to the isolated `internal/` packages to ensure a clean separation of concerns.

## Phase 1: Protocol & Interface Design
- [ ] **Define Protobuf Schema (`api/gobackup.proto`)**
  - Define core RPCs: `StartBackup`, `GetStatus`, `ListJobs`, `PruneBackups`.
  - Define the Streaming RPC: `rpc WatchLogs(WatchRequest) returns (stream LogChunk)` for real-time TUI updates.
- [ ] **Generate Go Code:** Setup `protoc` and generate the gRPC client/server stubs.

## Phase 2: The Core Daemon (Server)
- [ ] **Daemon Initialization:** Refactor `main.go` to support a `--daemon` flag that spins up the background 24/7 process.
- [ ] **Native Scheduling:** Replace Linux `cron` with an embedded Go scheduler (e.g., `robfig/cron/v3`). Load schedules directly from `config.yaml`.
- [ ] **Identity & Auth Store:** Implement a local SQLite or YAML database to securely store hashed user tokens and roles (Admin vs Viewer).
- [ ] **gRPC Interceptors:** Build the middleware to extract the Bearer Token from incoming gRPC metadata, validate the hash, and inject the user's identity/role into the request Context.
- [ ] **Action Logging:** Add secure audit logging (e.g., `[AUDIT] User 'jovens' triggered backup on kvm01`).

- [ ] **Dynamic Configuration Management:** Implement gRPC endpoints (e.g., `gbctl add host`) to dynamically register new servers to be backed up without manually editing files.
  - *Data Store Decision:* Adopt the "SourceVault Philosophy". The absolute Source of Truth will be individual YAML files inside a `conf.d/` directory to guarantee they never corrupt and can be version-controlled. The daemon will simply parse these files and cache them in-memory for fast execution.
  - *GitOps Integration:* Since the source of truth is a directory of YAML files, the daemon will optionally treat `conf.d/` as a Git repository. Adding a host via the CLI will automatically trigger a `git commit` and `git push` to back up the configuration to a remote Git server (SourceVault).

## Phase 3: The Thin Client (TUI)
- [ ] **Authentication UX:** Create the `gobackup login --token <TOKEN>` command to save credentials to `~/.config/gobackup/auth.json`.
- [ ] **Decouple Execution:** Strip all `os/exec` (tar/ssh) commands from the client. Route all actions through the gRPC client stubs.
- [ ] **Real-Time Log Streaming:** Wire the existing Tview/Bubbletea TUI to read from the gRPC stream instead of local `cmd.Stderr`. Ensure smooth rendering without UI blocking.

## Phase 4: Security & Networking
- [ ] **Transport Layer:** Implement TLS wrapping for the gRPC server (or bind strictly to Tailscale IPs).
- [ ] **Unix Socket Fallback:** Allow the daemon to listen on `/var/run/gobackup.sock` for zero-network, permission-based local execution.
