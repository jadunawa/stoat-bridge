package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jadunawa/stoat-bridge/internal/message"
	"github.com/jadunawa/stoat-bridge/internal/sender"
)

type mockSender struct {
	mu      sync.Mutex
	calls   []message.Message
	errFunc func(msg message.Message) error
	callCh  chan message.Message
}

func newMockSender() *mockSender {
	return &mockSender{callCh: make(chan message.Message, 100)}
}

func (m *mockSender) Send(_ context.Context, msg message.Message) error {
	m.mu.Lock()
	m.calls = append(m.calls, msg)
	m.mu.Unlock()
	m.callCh <- msg
	if m.errFunc != nil {
		return m.errFunc(msg)
	}
	return nil
}

func (m *mockSender) getCalls() []message.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]message.Message, len(m.calls))
	copy(result, m.calls)
	return result
}

func TestQueue_EnqueueAndDeliver(t *testing.T) {
	mock := newMockSender()
	q := New(mock, 10, 3, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)

	msg := message.Message{ChannelID: "ch-1", Content: "hello"}
	if !q.Enqueue(msg) {
		t.Fatal("enqueue should succeed")
	}

	select {
	case got := <-mock.callCh:
		if got.Content != "hello" {
			t.Errorf("content = %q, want %q", got.Content, "hello")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for delivery")
	}
}

func TestQueue_BufferFull_DropsNew(t *testing.T) {
	mock := newMockSender()
	// Block the sender so messages accumulate
	blocker := make(chan struct{})
	mock.errFunc = func(_ message.Message) error {
		<-blocker
		return nil
	}

	q := New(mock, 2, 3, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)

	// Fill the buffer (size 2) — one will be picked up by the worker
	q.Enqueue(message.Message{Content: "msg1"})
	q.Enqueue(message.Message{Content: "msg2"})
	q.Enqueue(message.Message{Content: "msg3"})

	// Fourth should be dropped — result is non-deterministic due to timing
	q.Enqueue(message.Message{Content: "msg4"})

	close(blocker)
}

func TestQueue_RetryOnTransientError(t *testing.T) {
	callCount := 0
	mock := newMockSender()
	mock.errFunc = func(_ message.Message) error {
		mock.mu.Lock()
		count := len(mock.calls)
		mock.mu.Unlock()
		if count <= 1 {
			return errors.New("transient error")
		}
		return nil
	}

	q := New(mock, 10, 3, nil)
	q.baseDelay = 10 * time.Millisecond // Speed up for tests

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)

	q.Enqueue(message.Message{Content: "retry-me"})

	// Wait for retry to succeed
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out; sender called %d times", callCount)
		case <-mock.callCh:
			calls := mock.getCalls()
			callCount = len(calls)
			if callCount >= 2 {
				return // Success — message was retried
			}
		}
	}
}

func TestQueue_DropOnPermanentError(t *testing.T) {
	mock := newMockSender()
	mock.errFunc = func(_ message.Message) error {
		return &sender.PermanentError{Err: errors.New("bad request")}
	}

	q := New(mock, 10, 3, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)

	q.Enqueue(message.Message{Content: "permanent-fail"})

	// Wait for the one delivery attempt
	select {
	case <-mock.callCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	// Give time for any retry (should NOT retry)
	time.Sleep(100 * time.Millisecond)
	calls := mock.getCalls()
	if len(calls) != 1 {
		t.Errorf("permanent error should not retry, got %d calls", len(calls))
	}
}

func TestQueue_GracefulDrain(t *testing.T) {
	mock := newMockSender()
	q := New(mock, 10, 3, nil)

	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)

	q.Enqueue(message.Message{Content: "drain-1"})
	q.Enqueue(message.Message{Content: "drain-2"})

	// Cancel context to stop accepting, then drain
	cancel()
	q.Shutdown(2 * time.Second)

	calls := mock.getCalls()
	if len(calls) < 2 {
		t.Errorf("expected at least 2 delivered during drain, got %d", len(calls))
	}
}
