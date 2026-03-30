package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("failed to encode response: %v", err)
	}
}

func newMockServer(t *testing.T, sendHandler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(r.URL.Path, "/getMe") {
			writeJSON(t, w, tgbotapi.APIResponse{
				Ok:     true,
				Result: json.RawMessage(`{"id":123,"is_bot":true,"first_name":"TestBot","username":"test_bot"}`),
			})
			return
		}

		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			if sendHandler != nil {
				sendHandler(w, r)
				return
			}
			writeJSON(t, w, tgbotapi.APIResponse{
				Ok:     true,
				Result: json.RawMessage(`{"message_id":1,"date":0,"chat":{"id":42,"type":"private"},"text":"ok"}`),
			})
			return
		}

		if strings.HasSuffix(r.URL.Path, "/getUpdates") {
			writeJSON(t, w, tgbotapi.APIResponse{
				Ok:     true,
				Result: json.RawMessage(`[]`),
			})
			return
		}

		http.NotFound(w, r)
	}))
}

func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint("test-token", serverURL+"/bot%s/%s")
	if err != nil {
		t.Fatalf("creating test bot: %v", err)
	}
	return &Client{bot: bot, chatID: 42}
}

func TestSendMessage_Success(t *testing.T) {
	var receivedText string
	var receivedParseMode string

	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		receivedText = r.FormValue("text")
		receivedParseMode = r.FormValue("parse_mode")

		writeJSON(t, w, tgbotapi.APIResponse{
			Ok:     true,
			Result: json.RawMessage(`{"message_id":1,"date":0,"chat":{"id":42,"type":"private"},"text":"ok"}`),
		})
	})
	defer server.Close()

	client := newTestClient(t, server.URL)
	err := client.SendMessage(context.Background(), "*Hello* world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedText != "*Hello* world" {
		t.Errorf("expected text %q, got %q", "*Hello* world", receivedText)
	}
	if receivedParseMode != "Markdown" {
		t.Errorf("expected parse mode %q, got %q", "Markdown", receivedParseMode)
	}
}

func TestSendMessage_RetryOnFailure(t *testing.T) {
	attempts := 0

	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			writeJSON(t, w, tgbotapi.APIResponse{
				Ok:          false,
				ErrorCode:   500,
				Description: "Internal Server Error",
			})
			return
		}
		writeJSON(t, w, tgbotapi.APIResponse{
			Ok:     true,
			Result: json.RawMessage(`{"message_id":1,"date":0,"chat":{"id":42,"type":"private"},"text":"ok"}`),
		})
	})
	defer server.Close()

	client := newTestClient(t, server.URL)
	err := client.SendMessage(context.Background(), "retry test")
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestSendMessage_AllRetriesFail(t *testing.T) {
	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, tgbotapi.APIResponse{
			Ok:          false,
			ErrorCode:   500,
			Description: "Internal Server Error",
		})
	})
	defer server.Close()

	client := newTestClient(t, server.URL)
	err := client.SendMessage(context.Background(), "fail test")
	if err == nil {
		t.Fatal("expected error after all retries failed")
	}
	if !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("expected retry exhaustion error, got: %v", err)
	}
}

func TestSendMessage_ContextCancelled(t *testing.T) {
	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, tgbotapi.APIResponse{
			Ok:          false,
			ErrorCode:   500,
			Description: "Internal Server Error",
		})
	})
	defer server.Close()

	client := newTestClient(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.SendMessage(ctx, "cancelled")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestStartPolling_GracefulShutdown(t *testing.T) {
	server := newMockServer(t, nil)
	defer server.Close()

	client := newTestClient(t, server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		client.StartPolling(ctx)
		close(done)
	}()

	select {
	case <-done:
		// polling stopped gracefully
	case <-time.After(3 * time.Second):
		t.Fatal("polling did not stop after context cancellation")
	}
}
