package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Saadlulu/dumptruckd/internal/utils"
	"github.com/Saadlulu/dumptruckd/pkg/config"
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
	if !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
		return nil, fmt.Errorf("webhook URL must use http:// or https:// scheme")
	}
	if strings.HasPrefix(cfg.URL, "http://") {
		if !cfg.AllowInsecure {
			return nil, fmt.Errorf("webhook URL %q uses plain HTTP; set allow_insecure = true to permit unencrypted notifications", cfg.URL)
		}
		slog.Warn("webhook URL uses plain HTTP — notification payloads will be sent unencrypted",
			"url", cfg.URL)
	}

	return &WebhookNotifier{
		url: cfg.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (n *WebhookNotifier) Notify(ctx context.Context, message string) error {
	payload := map[string]string{
		"message":   message,
		"timestamp": utils.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.url, bytes.NewBuffer(jsonData))
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
