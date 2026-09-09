# GoBackup (v2)

GoBackup is a blazing-fast, single-binary, client-server backup orchestration engine. It replaces complex local scripts with a robust gRPC daemon that natively manages SSH tar execution, cron scheduling, and GitOps configuration tracking.

## Architecture
- **`gobackupd`**: The background daemon (runs as a service). It holds an embedded SQLite database for auth, natively runs a Cron engine, and executes backups.
- **`gbctl`**: The thin client. It authenticates via gRPC, streams live logs, and manages the daemon's configuration without needing direct file access.

---

## 📦 Building & Installation

```bash
make build
sudo make install
```
*(This installs `gobackupd` and `gbctl` to `/usr/local/bin`)*

---

## 🏔️ Alpine Linux (OpenRC) Setup

To run `gobackupd` safely as a background service on Alpine Linux (running as the non-root `gobackup` user):

**1. Create the user & directories:**
```bash
sudo adduser -D gobackup
sudo mkdir -p /etc/gobackup /var/lib/gobackup
sudo chown gobackup:gobackup /etc/gobackup /var/lib/gobackup
```

**2. Install and start the OpenRC service:**
```bash
# Copy the init script from the repository
sudo cp alpine/gobackupd.initd /etc/init.d/gobackupd
sudo chmod +x /etc/init.d/gobackupd

# Enable it on boot
sudo rc-update add gobackupd default

# Start the daemon
sudo rc-service gobackupd start
```

---

## 🚀 CLI Usage Guide

### 1. Generate an Admin Token (God-Mode)
*If you are on the server hosting the daemon, you can bypass network auth using the local Unix socket to securely mint your first token:*
```bash
gbctl admin generate-token "your_username" "admin"
```

### 2. Authenticate the Client
```bash
gbctl login <YOUR_TOKEN> --server localhost:50051
```

### 3. Add a Backup Host
You can dynamically schedule and configure new backup targets directly from the CLI! The daemon will automatically write the YAML and push it to your GitOps repository.

```bash
# Add a host backing up /etc and /var/www every day at 2:00 AM
gbctl add host my-server 192.168.1.50 \
  --schedule "0 2 * * *" \
  --paths "/etc,/var/www" \
  --retention 10
```

### 4. Manually Run a Backup
Trigger a backup outside of its normal cron schedule and securely stream the live logs to your local terminal:
```bash
gbctl run my-server
```

### 5. View Configuration
List all the configured hosts and their cron schedules currently loaded in the daemon's memory:
```bash
gbctl list
```
