package sender

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jadunawa/stoat-bridge/internal/message"
)

func TestStoatSender_Success(t *testing.T) {
	var gotPath string
	var gotToken string
	var gotBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("x-bot-token")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewStoatSender(srv.URL, "test-token", srv.Client())
	err := s.Send(context.Background(), message.Message{
		ChannelID: "ch-123",
		Content:   "Hello, world!",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/channels/ch-123/messages" {
		t.Errorf("path = %q, want %q", gotPath, "/channels/ch-123/messages")
	}
	if gotToken != "test-token" {
		t.Errorf("token = %q, want %q", gotToken, "test-token")
	}
	if gotBody["content"] != "Hello, world!" {
		t.Errorf("content = %q, want %q", gotBody["content"], "Hello, world!")
	}
}

func TestStoatSender_TransientError_5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := NewStoatSender(srv.URL, "test-token", srv.Client())
	err := s.Send(context.Background(), message.Message{ChannelID: "ch", Content: "test"})

	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	var permErr *PermanentError
	if errors.As(err, &permErr) {
		t.Error("500 should be transient, not permanent")
	}
}

func TestStoatSender_TransientError_429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := NewStoatSender(srv.URL, "test-token", srv.Client())
	err := s.Send(context.Background(), message.Message{ChannelID: "ch", Content: "test"})

	if err == nil {
		t.Fatal("expected error for 429 response")
	}
	var permErr *PermanentError
	if errors.As(err, &permErr) {
		t.Error("429 should be transient, not permanent")
	}
}

func TestStoatSender_PermanentError_4xx(t *testing.T) {
	codes := []int{400, 401, 403, 404}
	for _, code := range codes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()

			s := NewStoatSender(srv.URL, "test-token", srv.Client())
			err := s.Send(context.Background(), message.Message{ChannelID: "ch", Content: "test"})

			if err == nil {
				t.Fatalf("expected error for %d response", code)
			}
			var permErr *PermanentError
			if !errors.As(err, &permErr) {
				t.Errorf("%d should be permanent error, got: %v", code, err)
			}
		})
	}
}

func TestStoatSender_NetworkError(t *testing.T) {
	s := NewStoatSender("http://localhost:1", "test-token", &http.Client{})
	err := s.Send(context.Background(), message.Message{ChannelID: "ch", Content: "test"})

	if err == nil {
		t.Fatal("expected error for network failure")
	}
	var permErr *PermanentError
	if errors.As(err, &permErr) {
		t.Error("network error should be transient, not permanent")
	}
}
