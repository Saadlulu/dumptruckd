package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

func TestNewWebhookNotifier_WithURL(t *testing.T) {
	notifier, err := NewWebhookNotifier(config.WebhookConfig{URL: "https://example.com/hook"})
	if err != nil {
		t.Fatalf("NewWebhookNotifier() error = %v", err)
	}
	if notifier == nil {
		t.Fatal("NewWebhookNotifier() returned nil")
	}
}

func TestNewWebhookNotifier_MissingURL(t *testing.T) {
	_, err := NewWebhookNotifier(config.WebhookConfig{})
	if err == nil {
		t.Error("NewWebhookNotifier() should error when URL is missing")
	}
}

func TestWebhookNotifier_Notify_SendsCorrectPayload(t *testing.T) {
	var receivedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		decoder := json.NewDecoder(r.Body)
		_ = decoder.Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := &WebhookNotifier{
		url:    server.URL,
		client: server.Client(),
	}

	err := notifier.Notify(context.Background(), "backup completed")
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	if receivedBody["message"] != "backup completed" {
		t.Errorf("Payload message = %q, want %q", receivedBody["message"], "backup completed")
	}
	if receivedBody["timestamp"] == "" {
		t.Error("Payload should include a timestamp")
	}
}

func TestWebhookNotifier_Notify_HandlesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	notifier := &WebhookNotifier{
		url:    server.URL,
		client: server.Client(),
	}

	err := notifier.Notify(context.Background(), "test")
	if err == nil {
		t.Error("Notify() should error on server error response")
	}
}

func TestWebhookNotifier_Notify_HandlesConnectionError(t *testing.T) {
	notifier := &WebhookNotifier{
		url:    "http://localhost:1",
		client: http.DefaultClient,
	}

	err := notifier.Notify(context.Background(), "test")
	if err == nil {
		t.Error("Notify() should error on connection failure")
	}
}

func TestNewWebhookNotifier_RejectsFileScheme(t *testing.T) {
	_, err := NewWebhookNotifier(config.WebhookConfig{URL: "file:///etc/passwd"})
	if err == nil {
		t.Error("NewWebhookNotifier() should reject file:// URLs")
	}
}

func TestNewWebhookNotifier_RejectsFTPScheme(t *testing.T) {
	_, err := NewWebhookNotifier(config.WebhookConfig{URL: "ftp://example.com/data"})
	if err == nil {
		t.Error("NewWebhookNotifier() should reject ftp:// URLs")
	}
}
