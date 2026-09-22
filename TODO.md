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
  * `EmailNotifier`: Handles SMTP dial and generic HTML/Text email generation.
  * `GenericWebhookNotifier`: Handles standard HTTP POSTs with a clean JSON schema for generic integrations.

## 3. Execution Engine Updates
* Update `SendNotification` inside `internal/engine/backup.go` and `internal/engine/notifications.go` to accept the new configuration array.
* Iterate through all configured providers dynamically and execute `.Send()` concurrently using goroutines (`go provider.Send(...)`) so slow SMTP servers don't block backup completion logic.

## 4. CLI / API Integration
* Expand the `gbctl` admin tools or configuration loader to safely parse, validate, and test these multi-channel configurations.
