package engine

import (
	"io"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Notifier defines the unified interface for all notification channels.
type Notifier interface {
	Send(title, description, host, targetFile string, color int) error
}

// ============================================================================
// GOTCODE DISPATCH NOTIFIER (LEGACY)
// ============================================================================

type GotcodeDispatchNotifier struct {
	WebhookURL string
}

func (g *GotcodeDispatchNotifier) Send(title, description, host, targetFile string, color int) error {
	if g.WebhookURL == "" {
		return nil
	}

	payload := map[string]interface{}{
		"title":       title,
		"description": description,
		"color":       color,
		"fields": [][]interface{}{
			{"Backup Server", "GoBackup-Daemon", true},
			{"Remote Target", host, true},
			{"Destination", targetFile, false},
		},
		"footer": "GoBackup Automated Task",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil { return err }

	req, err := http.NewRequest("POST", g.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()

	if resp.StatusCode >= 400 { return fmt.Errorf("gotcode_dispatch webhook returned status %d", resp.StatusCode) }
	return nil
}

// ============================================================================
// DISCORD NOTIFIER
// ============================================================================

type DiscordNotifier struct {
	WebhookURL string
}

func (d *DiscordNotifier) Send(title, description, host, targetFile string, color int) error {
	if d.WebhookURL == "" {
		return nil
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       title,
				"description": description,
				"color":       color,
				"timestamp":   time.Now().Format(time.RFC3339),
				"fields": []map[string]interface{}{
					{"name": "Host", "value": host, "inline": true},
					{"name": "Target", "value": targetFile, "inline": false},
				},
				"footer": map[string]interface{}{
					"text": "GoBackup Core Engine",
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil { return err }

	req, err := http.NewRequest("POST", d.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()

	if resp.StatusCode >= 400 { return fmt.Errorf("discord webhook returned status %d", resp.StatusCode) }
	return nil
}

// ============================================================================
// EMAIL NOTIFIER (STUB)
// ============================================================================

type EmailNotifier struct {
	Host string
	Port int
	User string
	Pass string
	To   string
	From string
}

func (e *EmailNotifier) Send(title, description, host, targetFile string, color int) error {
	log.Printf("[DEBUG] EmailNotifier stub hit. Would send email to %s regarding %s.", e.To, host)
	return nil
}

// ============================================================================
// M365 GRAPH API NOTIFIER
// ============================================================================

type M365GraphNotifier struct {
	TenantID string
	ClientID string
	Secret   string
	To       string
	From     string
}

type graphTokenResponse struct {
	AccessToken string `json:"access_token"`
}

func (m *M365GraphNotifier) Send(title, description, host, targetFile string, color int) error {
	if m.TenantID == "" || m.ClientID == "" || m.Secret == "" || m.To == "" || m.From == "" {
		return fmt.Errorf("missing required M365 Graph API configuration parameters")
	}

	// 1. Authenticate via OAuth2 Client Credentials Flow
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", m.TenantID)
	authBody := fmt.Sprintf("client_id=%s&scope=https%%3A%%2F%%2Fgraph.microsoft.com%%2F.default&client_secret=%s&grant_type=client_credentials", 
		m.ClientID, m.Secret)

	reqAuth, err := http.NewRequest("POST", tokenURL, strings.NewReader(authBody))
	if err != nil { return err }
	reqAuth.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	respAuth, err := client.Do(reqAuth)
	if err != nil { return err }
	defer respAuth.Body.Close()

	if respAuth.StatusCode >= 400 {
		return fmt.Errorf("m365 auth failed with status %d", respAuth.StatusCode)
	}

	var tokenRes graphTokenResponse
	if err := json.NewDecoder(respAuth.Body).Decode(&tokenRes); err != nil {
		return err
	}

	// 2. Construct the Email Payload
	statusHtml := "Completed Successfully"
	if color == 0x3498DB { // Blue
		statusHtml = "<span style='color:blue;'>Started...</span>"
	} else if color == 0xF1C40F { // Yellow/Warning
		statusHtml = "<strong style='color:orange;'>WARNING</strong>"
	} else if color == 0xFF0000 { // Red
		statusHtml = "<strong style='color:red;'>FAILED</strong>"
	} else if color == 0x00FF00 { // Green
		statusHtml = "<strong style='color:green;'>SUCCESS</strong>"
	}

	emailPayload := map[string]interface{}{
		"message": map[string]interface{}{
			"subject": title,
			"body": map[string]interface{}{
				"contentType": "HTML",
				"content": fmt.Sprintf(`
					<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; max-width: 600px; margin: 0 auto; background-color: #f9fafb; padding: 20px; border-radius: 8px;">
						<div style="background-color: #ffffff; border-left: 6px solid #4f46e5; border-radius: 6px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); padding: 24px;">
							<div style="border-bottom: 1px solid #e5e7eb; padding-bottom: 16px; margin-bottom: 20px;">
								<h2 style="margin: 0; color: #111827; font-size: 20px; font-weight: 600;">GoBackup Alert</h2>
								<p style="margin: 6px 0 0; color: #6b7280; font-size: 14px;">%s</p>
							</div>
							
							<div style="margin-bottom: 20px;">
								<table style="width: 100%%; border-collapse: collapse;">
									<tr>
										<td style="padding: 8px 0; color: #6b7280; font-size: 13px; font-weight: 600; text-transform: uppercase; width: 120px;">Target Host</td>
										<td style="padding: 8px 0; color: #111827; font-size: 15px; font-weight: 500;">%s</td>
									</tr>
									<tr>
										<td style="padding: 8px 0; color: #6b7280; font-size: 13px; font-weight: 600; text-transform: uppercase;">Archive Path</td>
										<td style="padding: 8px 0; color: #111827; font-size: 14px; font-family: monospace; background: #f3f4f6; padding: 4px 8px; border-radius: 4px; word-break: break-all;">%s</td>
									</tr>
									<tr>
										<td style="padding: 8px 0; color: #6b7280; font-size: 13px; font-weight: 600; text-transform: uppercase;">Current Status</td>
										<td style="padding: 8px 0; font-size: 15px;">%s</td>
									</tr>
								</table>
							</div>
							
							<div style="border-top: 1px solid #e5e7eb; padding-top: 16px; text-align: center;">
								<p style="margin: 0; color: #9ca3af; font-size: 12px;">This is an automated message from the GoBackup Daemon.</p>
							</div>
						</div>
					</div>
				`, description, host, targetFile, statusHtml),
			},
			"toRecipients": []map[string]interface{}{
				{
					"emailAddress": map[string]interface{}{
						"address": m.To,
					},
				},
			},
		},
		"saveToSentItems": "false",
	}

	jsonData, err := json.Marshal(emailPayload)
	if err != nil { return err }

	// 3. Dispatch the Email via Graph API
	sendURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/sendMail", m.From)
	reqSend, err := http.NewRequest("POST", sendURL, bytes.NewBuffer(jsonData))
	if err != nil { return err }
	
	reqSend.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	reqSend.Header.Set("Content-Type", "application/json")

	respSend, err := client.Do(reqSend)
	if err != nil { return err }
	defer respSend.Body.Close()

	if respSend.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(respSend.Body)
		return fmt.Errorf("m365 sendMail failed with status %d: %s", respSend.StatusCode, string(bodyBytes))
	}

	return nil
}

// ============================================================================
// GENERIC WEBHOOK NOTIFIER (STUB)
// ============================================================================

type GenericWebhookNotifier struct {
	URL string
}

func (g *GenericWebhookNotifier) Send(title, description, host, targetFile string, color int) error {
	log.Printf("[DEBUG] GenericWebhookNotifier stub hit. Would POST JSON to %s.", g.URL)
	return nil
}

// ============================================================================
// NOTIFICATION DISPATCHER
// ============================================================================

// SendNotifications dynamically parses the array of notification configs and fires them concurrently.
func SendNotifications(configs []NotificationConfig, title, description, host, targetFile string, color int) {
	var notifiers []Notifier

	for _, cfg := range configs {
		switch strings.ToLower(cfg.Type) {
		case "discord":
			notifiers = append(notifiers, &DiscordNotifier{WebhookURL: cfg.URL})
		case "gotcode_dispatch":
			notifiers = append(notifiers, &GotcodeDispatchNotifier{WebhookURL: cfg.URL})
		case "email", "smtp":
			notifiers = append(notifiers, &EmailNotifier{
				Host: cfg.SMTPHost, Port: cfg.SMTPPort,
				User: cfg.SMTPUser, Pass: cfg.SMTPPass,
				To: cfg.To, From: cfg.From,
			})
		case "m365_graph", "m365":
			notifiers = append(notifiers, &M365GraphNotifier{
				TenantID: cfg.TenantID, ClientID: cfg.ClientID, Secret: cfg.Secret,
				To: cfg.To, From: cfg.From,
			})
		case "webhook", "http":
			notifiers = append(notifiers, &GenericWebhookNotifier{URL: cfg.URL})
		}
	}

	// Fire them concurrently so slow SMTP relays don't block the global backup queue!
	for _, n := range notifiers {
		go func(notifier Notifier) {
			if err := notifier.Send(title, description, host, targetFile, color); err != nil {
				log.Printf("⚠️ Notification failed: %v", err)
			}
		}(n)
	}
}
