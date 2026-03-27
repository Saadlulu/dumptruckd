package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/dumptruckd/dumptruckd/pkg/config"
)

func TestNewSlackNotifier_WithConfigURL(t *testing.T) {
	notifier, err := NewSlackNotifier(config.SlackConfig{
		WebhookURL: "https://hooks.slack.com/test",
	})
	if err != nil {
		t.Fatalf("NewSlackNotifier() error = %v", err)
	}
	if notifier == nil {
		t.Fatal("NewSlackNotifier() returned nil")
	}
}

func TestNewSlackNotifier_FallsBackToEnvVar(t *testing.T) {
	os.Setenv("SLACK_WEBHOOK_URL", "https://hooks.slack.com/from-env")
	defer os.Unsetenv("SLACK_WEBHOOK_URL")

	notifier, err := NewSlackNotifier(config.SlackConfig{})
	if err != nil {
		t.Fatalf("NewSlackNotifier() error = %v", err)
	}
	if notifier.webhookURL != "https://hooks.slack.com/from-env" {
		t.Errorf("webhookURL = %q, want env var value", notifier.webhookURL)
	}
}

func TestNewSlackNotifier_MissingURL(t *testing.T) {
	os.Unsetenv("SLACK_WEBHOOK_URL")

	_, err := NewSlackNotifier(config.SlackConfig{})
	if err == nil {
		t.Error("NewSlackNotifier() should error when no webhook URL is available")
	}
}

func TestSlackNotifier_Notify_SendsCorrectPayload(t *testing.T) {
	var receivedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		decoder := json.NewDecoder(r.Body)
		decoder.Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := &SlackNotifier{
		webhookURL: server.URL,
		client:     server.Client(),
	}

	err := notifier.Notify("test message")
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	if receivedBody["text"] != "test message" {
		t.Errorf("Payload text = %q, want %q", receivedBody["text"], "test message")
	}
}

func TestSlackNotifier_Notify_HandlesNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := &SlackNotifier{
		webhookURL: server.URL,
		client:     server.Client(),
	}

	err := notifier.Notify("test message")
	if err == nil {
		t.Error("Notify() should error on non-200 response")
	}
}

func TestSlackNotifier_Notify_HandlesConnectionError(t *testing.T) {
	notifier := &SlackNotifier{
		webhookURL: "http://localhost:1", // nothing listening
		client:     http.DefaultClient,
	}

	err := notifier.Notify("test message")
	if err == nil {
		t.Error("Notify() should error on connection failure")
	}
}
