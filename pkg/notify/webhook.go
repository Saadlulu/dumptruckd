package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dumptruckd/dumptruckd/pkg/config"
)

// WebhookNotifier sends notifications via generic HTTP POST webhooks.
type WebhookNotifier struct {
	url    string
	client *http.Client
}

// NewWebhookNotifier creates a webhook notifier for the given URL.
func NewWebhookNotifier(cfg config.WebhookConfig) (*WebhookNotifier, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	return &WebhookNotifier{
		url: cfg.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (n *WebhookNotifier) Notify(message string) error {
	payload := map[string]string{
		"message": message,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", n.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

