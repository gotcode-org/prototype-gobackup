# GoBackup Installation Guide

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
