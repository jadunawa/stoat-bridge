package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jadunawa/stoat-bridge/internal/message"
)

type StoatSender struct {
	apiURL   string
	botToken string
	client   *http.Client
}

func NewStoatSender(apiURL, botToken string, client *http.Client) *StoatSender {
	return &StoatSender{
		apiURL:   apiURL,
		botToken: botToken,
		client:   client,
	}
}

func (s *StoatSender) Send(ctx context.Context, msg message.Message) error {
	url := fmt.Sprintf("%s/channels/%s/messages", s.apiURL, msg.ChannelID)

	body, err := json.Marshal(map[string]string{"content": msg.Content})
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bot-token", s.botToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return fmt.Errorf("transient error: HTTP %d", resp.StatusCode)
	}

	return &PermanentError{Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
}
