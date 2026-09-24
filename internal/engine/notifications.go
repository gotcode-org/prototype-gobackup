package engine

import (
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
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", d.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}
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
// M365 GRAPH API NOTIFIER (STUB)
// ============================================================================

type M365GraphNotifier struct {
	TenantID string
	ClientID string
	Secret   string
	To       string
	From     string
}

func (m *M365GraphNotifier) Send(title, description, host, targetFile string, color int) error {
	log.Printf("[DEBUG] M365GraphNotifier stub hit. Would POST to Microsoft Graph for %s.", host)
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
		case "discord", "slack":
			notifiers = append(notifiers, &DiscordNotifier{WebhookURL: cfg.URL})
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
