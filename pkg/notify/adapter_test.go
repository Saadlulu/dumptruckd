package notify

import (
	"os"
	"strings"
	"testing"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

func TestNewNotifier(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.NotifyConfig
		envVars map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "slack with webhook url in config",
			cfg: config.NotifyConfig{
				Type:  "slack",
				Slack: config.SlackConfig{WebhookURL: "https://hooks.slack.com/test"},
			},
			wantErr: false,
		},
		{
			name: "slack with webhook url from env",
			cfg: config.NotifyConfig{
				Type:  "slack",
				Slack: config.SlackConfig{},
			},
			envVars: map[string]string{"SLACK_WEBHOOK_URL": "https://hooks.slack.com/test"},
			wantErr: false,
		},
		{
			name: "slack missing webhook url",
			cfg: config.NotifyConfig{
				Type:  "slack",
				Slack: config.SlackConfig{},
			},
			wantErr: true,
			errMsg:  "webhook URL is required",
		},
		{
			name: "webhook with url",
			cfg: config.NotifyConfig{
				Type:    "webhook",
				Webhook: config.WebhookConfig{URL: "https://example.com/hook"},
			},
			wantErr: false,
		},
		{
			name: "webhook missing url",
			cfg: config.NotifyConfig{
				Type:    "webhook",
				Webhook: config.WebhookConfig{},
			},
			wantErr: true,
			errMsg:  "URL is required",
		},
		{
			name:    "none",
			cfg:     config.NotifyConfig{Type: "none"},
			wantErr: false,
		},
		{
			name:    "email not implemented",
			cfg:     config.NotifyConfig{Type: "email"},
			wantErr: true,
			errMsg:  "not yet implemented",
		},
		{
			name:    "discord not implemented",
			cfg:     config.NotifyConfig{Type: "discord"},
			wantErr: true,
			errMsg:  "not yet implemented",
		},
		{
			name:    "unknown type",
			cfg:     config.NotifyConfig{Type: "telegram"},
			wantErr: true,
			errMsg:  "unknown notification type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env first
			os.Unsetenv("SLACK_WEBHOOK_URL")

			// Set env vars for this test
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			_, err := NewNotifier(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewNotifier() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("NewNotifier() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
