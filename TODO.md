# Phase 4: Advanced Notifications

## 1. Configuration Engine Updates
* Modify the root `Config` struct to replace the single `WebhookURL` string with an array of `NotificationProvider` configurations.
* Support multiple concurrent channels natively. Example YAML structure:
  ```yaml
  notifications:
    - type: discord
      url: "https://discord.com/api/webhooks/..."
    - type: email
      smtp_host: "smtp.gmail.com"
      smtp_port: 587
      smtp_user: "alerts@example.com"
      smtp_pass: "secret"
      to: "admin@example.com"
      from: "gobackup@example.com"
    - type: webhook
      url: "https://my-custom-endpoint.com/alerts"
  ```

## 2. Notification Provider Interface
* Define a unified `Notifier` interface in Go:
  ```go
  type Notifier interface {
      Send(title, description, host, targetFile string, color int) error
  }
  ```
* Implement concrete providers that satisfy the interface:
  * `DiscordNotifier`: Handles the existing Discord/Slack JSON payloads.
  * `EmailNotifier`: Handles traditional SMTP dialing (e.g. internal unauthenticated relays or standard TLS).
  * `M365GraphNotifier`: Authenticates via Microsoft Graph API (OAuth2) to natively send emails via Office 365 without relying on legacy SMTP.
  * `GenericWebhookNotifier`: Handles standard HTTP POSTs with a clean JSON schema for generic integrations.

## 3. Execution Engine Updates
* Update `SendNotification` inside `internal/engine/backup.go` and `internal/engine/notifications.go` to accept the new configuration array.
* Iterate through all configured providers dynamically and execute `.Send()` concurrently using goroutines (`go provider.Send(...)`) so slow SMTP servers don't block backup completion logic.

## 4. CLI / API Integration
* Expand the `gbctl` admin tools or configuration loader to safely parse, validate, and test these multi-channel configurations.

# Phase 5: Cold Storage Archiving

## 1. Multi-Tier Retention Configuration
* Update `JobConfig` to support an optional cold storage tier:
  ```yaml
  cold_storage:
    path: "/mnt/nfs-cold-storage"
    retention_chains: 12  # Keep 12 chains (e.g., 12 weeks) in cold storage
  ```

## 2. Pruning Engine Upgrades
* In `CleanupOldBackups()`, instead of immediately issuing an `os.Remove()` when a chain exceeds the hot `RetentionCount`, check if `cold_storage.path` is defined.
* If defined, natively move (`os.Rename` or `io.Copy` if cross-device) the entire expiring chain (the `[FULL]` anchor and all attached `[INC]` files) to the cold storage mount.

## 3. Cold Storage Lifecycle Management
* Build a secondary sweep function (`CleanupColdStorage()`) that runs immediately after the hot sweep.
* This sweep will chronologically group chains inside the cold storage volume and definitively `os.Remove()` them once they exceed the `cold_storage.retention_chains` limit.

## 4. CLI Transparency
* Update `gbctl backup list` so it can scan both the Hot volume and the Cold volume, visually indicating to the user whether an archive is currently in fast local storage or deep cold storage.

# Phase 6: Native Docker Volume Restoration

## 1. API and CLI Enhancements
* Update the `RestoreBackupRequest` protobuf and the `gbctl backup restore` CLI to accept a new flag like `--target-volume <volume_name>`.
* If a user specifies `--target-volume`, the daemon should bypass the standard directory extraction logic and instead route the restore stream directly into the Docker engine.

## 2. Docker Pipeline Injection
* When restoring directly to a named Docker volume, the engine will automatically spin up an ephemeral `alpine` container with the target volume mounted to `/dest`.
* The standard chronological tar stream (FULL + INCs) will be piped directly over SSH into the ephemeral container's `tar -xzf - -C /dest` extraction pipeline.
* This ensures all ownership (`root:root`, etc.) and permissions are perfectly restored inside the Docker daemon's managed `/var/lib/docker/volumes` namespace without requiring `sudo` privileges on the host filesystem.

# Phase 7: CLI Dashboard Formatting & Polish

## 1. gbctl status Queued Jobs Refactor
* Right now, `gbctl status` dumps all dynamically queued jobs into a comma-separated string `⏳ Queued Jobs (14): a, b, c...` which causes ugly terminal line-wrapping when many jobs are stacked.
* Update `cmd/gbctl/status.go` to parse the `resp.QueuedJobs` array and output it using Go's `text/tabwriter` engine.
* Format it nicely into columns: `QUEUE POS | SERVER | JOB`. 
* Ensure string splitting correctly identifies the server vs the job name (splitting the tracking ID on the first underscore).

## 2. gbctl status Scheduled Jobs Limit
* `gbctl status` currently hardcodes a limit of 15 scheduled jobs before truncating the output (`... and X more`).
* Add a `--all` or `--limit` flag to `gbctl status` so the user can easily print the entire cron-ordered schedule regardless of how massive the fleet is.

# Phase 8: Storage Resilience & Mount Verification

## 1. Sentinel File Verification
* Prevent the daemon from accidentally filling up the local root filesystem (`/`) if a Network File System (NFS) drops.
* The engine should require a sentinel file (e.g., `.gobackup_mounted`) to exist at the root of both the Hot `backup_dir` and the Cold Storage `cold_storage_path`.
* If the engine tries to initiate a backup or a prune and this file is missing, it instantly aborts and fires a critical webhook notification: `Backup Failed: Target storage volume is unmounted.`

## 2. Stale NFS Handle Detection
* NFS handles can become "stale" if the upstream TrueNAS/Synology server reboots, which ordinarily causes standard Linux commands (like `stat` or `ls`) to hang infinitely, freezing the backup queue.
* Implement a heavily timeout-bounded stat check (e.g., using Go's `context.WithTimeout(..., 3*time.Second)`) against the sentinel file before starting any I/O operations.
* If the I/O check times out or returns an `ESTALE` error, the engine must trap the failure, gracefully abort the job without freezing the daemon, and send a webhook notification: `Backup Failed: NFS Mount is stale or unresponsive.`

## 3. gbctl attach Log Prefix Formatting
* When running `gbctl attach` (or using `--attach` on job runs), the stream currently prefixes log lines with `[job-name]`.
* Update the logging engine to namespace the prefix as `[server_name@job_name]` so it's explicitly clear which remote host the output belongs to (especially useful for parallel or similarly-named jobs).

## 5. Documentation (docs/NOTIFICATIONS.md)
* Create a dedicated `docs/NOTIFICATIONS.md` file to thoroughly document the multi-channel notification engine.
* Include specific examples on how to configure `gobackup.yaml` for each provider.
* Crucially, include external tutorials on how to provision these endpoints (e.g., how to register an Azure AD App for M365 Graph API, how to create a Discord Webhook, etc).

## 6. Notification Digests & Queue Batching
* Firing individual start/finish webhooks for 20 jobs that take 3 seconds each creates severe alert fatigue.
* Implement a `BatchTracker` in the global Mutex engine. When the cron scheduler fires multiple jobs simultaneously (e.g., at `0 2 * * *`), group them into a single "Run Session".
* Emit a single **"Backup Queue Initiated"** alert explicitly listing all of the `server_name/job_name` targets that were just added to the queue, so the user knows exactly what is running in this batch.
* As jobs complete, silently tally their results (Success, Warning, Failure, Duration, Size) in memory.
* When the queue empties and returns to `Idle`, emit a single consolidated **"Backup Run Digest"** alert featuring a clean summary report of all jobs processed in that batch. This digest must explicitly list the `server_name/job_name` grouped under clear headers for ✅ Success, ⚠️ Warnings, and ❌ Failures so the user knows exactly which servers had issues at a glance.
