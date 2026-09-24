# GoBackup Notifications Guide

GoBackup supports a highly concurrent, multi-channel notification engine. Instead of defining a single webhook in your configuration, you define an array of `notifications`. GoBackup will dispatch alerts to all defined providers simultaneously without blocking the backup queue.

---

## 1. Microsoft 365 Graph API (`type: m365`)

Because Microsoft has aggressively deprecated standard SMTP Auth across Exchange Online, GoBackup natively supports the **Microsoft Graph API**. It authenticates securely via an OAuth2 Client Credentials flow, entirely bypassing legacy SMTP.

### Configuration
```yaml
notifications:
  - type: m365
    tenant_id: "your-azure-tenant-id"
    client_id: "your-azure-app-client-id"
    secret: "your-client-secret-value"
    from: "alerts@yourdomain.com"
    to: "admin@yourdomain.com"
```

### Setup Guide (Azure Entra ID / Active Directory)
To get these credentials, you must register a headless application in your Microsoft 365 tenant:

1. **Register the App:**
   * Go to the [Microsoft Entra Admin Center](https://entra.microsoft.com/) -> **Applications** -> **App registrations**.
   * Click **New registration** (Name it "GoBackup Alerts").
   * Copy the **Application (client) ID** and **Directory (tenant) ID** from the Overview page.
2. **Grant API Permissions:**
   * Go to **API permissions** -> **Add a permission** -> **Microsoft Graph** -> **Application permissions** (NOT Delegated).
   * Search for and select `Mail.Send`.
   * **Crucial:** Click **"Grant admin consent for <Your Tenant>"** so the status turns into a green checkmark.
3. **Generate a Secret:**
   * Go to **Certificates & secrets** -> **New client secret**.
   * Copy the generated **Value** (this is your `secret`). You will only see this once.

*Note: The `from` email address must be a valid, licensed mailbox (or shared mailbox) inside your M365 tenant.*

---

## 2. GotCode Dispatch Webhook (`type: gotcode_dispatch`)

A dedicated webhook handler that dispatches a legacy, flat JSON structure specifically built for GotCode's internal intermediary alert routers. 

### Configuration
```yaml
notifications:
  - type: gotcode_dispatch
    url: "https://discord-wh.gotunix.net/webhook/custom?token=YOUR_TOKEN"
```

---

## 3. Standard Discord Webhook (`type: discord`)

Sends native, embed-formatted payloads directly to a Discord or Slack channel webhook.

### Configuration
```yaml
notifications:
  - type: discord
    url: "https://discord.com/api/webhooks/123456789/ABCDEFG"
```

### Setup Guide (Discord)
1. Right-click your Discord server channel -> **Edit Channel**.
2. Go to **Integrations** -> **Webhooks** -> **New Webhook**.
3. Name it "GoBackup" and click **Copy Webhook URL**.

---

## 4. Standard SMTP Email (`type: email`)

Dispatches modern, HTML-formatted email alerts using standard `net/smtp` dialing. It natively supports both unauthenticated internal relays (like Postfix on port 25) and standard TLS-authenticated relays (like Gmail or SendGrid on port 587).

### Configuration
```yaml
notifications:
  - type: email
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    smtp_user: "alerts@yourdomain.com" # Optional (Remove for unauthenticated relays)
    smtp_pass: "your-app-password"     # Optional
    to: "admin@yourdomain.com"
    from: "alerts@yourdomain.com"
```

---

## 5. Generic HTTP Webhook (`type: webhook`)

Dispatches a clean, highly parsable flat JSON payload to any HTTP endpoint. Perfect for custom APIs, n8n, Zapier, or Make.com integrations where Discord's heavy "embed" formatting isn't needed.

### Configuration
```yaml
notifications:
  - type: webhook
    url: "https://your-custom-api.com/webhook/receive"
```

### Example Payload
```json
{
  "title": "GoBackup Alert",
  "description": "Initiating tar pull natively...",
  "host": "server_name",
  "target_file": "/path/to/archive.tar.gz",
  "status": "success",
  "color": 65280,
  "timestamp": "2026-09-24T15:00:00Z"
}
```
