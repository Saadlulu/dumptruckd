package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// SlackNotifier sends notifications via Slack incoming webhooks.
type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

// NewSlackNotifier creates a Slack notifier from config or SLACK_WEBHOOK_URL env var.
func NewSlackNotifier(cfg config.SlackConfig) (*SlackNotifier, error) {
	webhookURL := cfg.WebhookURL
	if webhookURL == "" {
		webhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	}
	if webhookURL == "" {
		return nil, fmt.Errorf("slack webhook URL is required (config or SLACK_WEBHOOK_URL env var)")
	}
	if !strings.HasPrefix(webhookURL, "https://") {
		return nil, fmt.Errorf("slack webhook URL must use https://")
	}

	return &SlackNotifier{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (n *SlackNotifier) Notify(ctx context.Context, message string) error {
	payload := map[string]string{
		"text": message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API returned status %d", resp.StatusCode)
	}

	return nil
}

