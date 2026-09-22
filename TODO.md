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
