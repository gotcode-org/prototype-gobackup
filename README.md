# GoBackup

GoBackup is a blazing-fast, single-binary, client-server backup orchestration engine. It replaces complex local scripts with a robust gRPC daemon that natively manages SSH tar execution, cron scheduling, incremental diffs, and GitOps configuration tracking.

## 🚀 Features

* **Zero-Dependency Native Execution**: Runs purely over standard SSH using native GNU `tar`. No need to install agents, Borg, Restic, or rsyncd on the target servers.
* **Smart Incremental Chains**: Fully supports `--listed-incremental` (`-g`) tarball chains. The engine automatically rotates Full and Incremental backups based on interval schedules and seamlessly prunes orphaned chains to maintain healthy retention policies.
* **Dual-Tier Archiving (Cold Storage)**: Dynamically ferry expired local backups (Hot Storage) to cheap, high-capacity NFS or cloud mount points (Cold Storage). Native support for job-level path routing and strict chain-retention pruning.
* **Docker Volume Aware**: Automatically spins up disposable utility containers to backup live Docker volumes securely, even traversing files owned by root, without needing to map unprivileged UIDs.
* **Reverse-Stream Restorations**: Select a backup from the CLI, and the engine will seamlessly trace the chain back to the anchor `[FULL]` backup and sequentially stream the tarballs backwards over an SSH pipe to perfectly rebuild the filesystem state on the target machine.
* **Live TUI Log Streaming**: Trigger backup jobs manually from the CLI and instantly attach to the daemon's live log stream to watch the execution in real time.
* **GitOps & Cron Integration**: The `gobackupd` daemon features a built-in cron scheduler and an embedded SQLite database. Configurations dynamically created via the `gbctl` CLI are automatically pushed to your GitOps repository for version-controlled infrastructure.

## 🏗 Architecture

GoBackup is architected using a decoupled Client-Server model:

1. **`gobackupd` (The Daemon)**: A background service that holds an embedded database for authentication, natively runs the Cron engine, manages snapshot states, and executes the heavy-lifting SSH tar backup processes.
2. **`gbctl` (The CLI Client)**: A thin, noun-verb styled CLI tool that authenticates securely via gRPC. It manages the daemon's configuration, manually fires off jobs, traces restoration chains, and streams live logs without needing direct file access.

## 📚 Documentation

- [Installation Guide](docs/INSTALL.md)

## ⚡ Quick Start

```bash
# Generate a local admin token securely via unix sockets
gbctl admin generate-token "your_username" "admin"

# Authenticate your thin client
gbctl login <YOUR_TOKEN> --server localhost:50051

# Add a backup server
gbctl server add dev-server 192.168.1.50 --port 22 --user backup --sudo

# Configure a job to backup /etc and a docker volume incrementally
gbctl job add dev-server app-backup \
  --schedule "0 2 * * *" \
  --paths "/etc,/var/www" \
  --docker-volumes "postgres-data" \
  --incremental \
  --full-interval 7 \
  --retention 4

# Run the job manually and instantly attach to the live logs
gbctl job run dev-server app-backup --attach
```
