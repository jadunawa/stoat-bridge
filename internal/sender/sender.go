package sender

import (
	"context"
	"fmt"

	"github.com/jadunawa/stoat-bridge/internal/message"
)

// Sender delivers a message to a chat platform.
type Sender interface {
	Send(ctx context.Context, msg message.Message) error
}

// PermanentError indicates a non-retryable failure.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return fmt.Sprintf("permanent error: %v", e.Err)
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}
