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

*(In Development)* - Will support standard unauthenticated port 25 relays (e.g. Postfix) and standard TLS authenticated relays.

## 5. Generic HTTP Webhook (`type: webhook`)

*(In Development)* - Will dispatch a clean, standard JSON payload (`{"status": "success", "job": "..."}`) for generic integrations like n8n or Zapier.
