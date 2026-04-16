package queue

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jadunawa/stoat-bridge/internal/message"
	"github.com/jadunawa/stoat-bridge/internal/sender"
)

type item struct {
	msg      message.Message
	attempts int
}

type Queue struct {
	ch         chan *item
	sender     sender.Sender
	maxRetries int
	logger     *slog.Logger
	baseDelay  time.Duration
	wg         sync.WaitGroup
	done       chan struct{}
}

func New(s sender.Sender, size, maxRetries int, logger *slog.Logger) *Queue {
	if logger == nil {
		logger = slog.Default()
	}
	return &Queue{
		ch:         make(chan *item, size),
		sender:     s,
		maxRetries: maxRetries,
		logger:     logger,
		baseDelay:  1 * time.Second,
		done:       make(chan struct{}),
	}
}

func (q *Queue) Enqueue(msg message.Message) bool {
	select {
	case q.ch <- &item{msg: msg, attempts: 0}:
		return true
	default:
		q.logger.Warn("queue full, message dropped",
			"channel_id", msg.ChannelID,
		)
		return false
	}
}

func (q *Queue) Depth() int {
	return len(q.ch)
}

func (q *Queue) Start(ctx context.Context) {
	q.wg.Add(1)
	go q.worker(ctx)
}

func (q *Queue) Shutdown(timeout time.Duration) {
	close(q.done)

	// Drain remaining items with timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case it := <-q.ch:
			if it == nil {
				return
			}
			q.deliver(context.Background(), it)
		case <-timer.C:
			q.logger.Warn("shutdown timeout, some messages may be lost",
				"remaining", len(q.ch),
			)
			return
		default:
			return
		}
	}
}

func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()
	for {
		select {
		case <-q.done:
			return
		case <-ctx.Done():
			return
		case it := <-q.ch:
			if it == nil {
				return
			}
			q.deliver(ctx, it)
		}
	}
}

func (q *Queue) deliver(ctx context.Context, it *item) {
	it.attempts++
	err := q.sender.Send(ctx, it.msg)
	if err == nil {
		q.logger.Debug("message delivered",
			"channel_id", it.msg.ChannelID,
			"attempts", it.attempts,
		)
		return
	}

	var permErr *sender.PermanentError
	if errors.As(err, &permErr) {
		q.logger.Error("permanent delivery failure, message dropped",
			"channel_id", it.msg.ChannelID,
			"error", err,
		)
		return
	}

	if it.attempts >= q.maxRetries {
		q.logger.Warn("max retries exhausted, message dropped",
			"channel_id", it.msg.ChannelID,
			"attempts", it.attempts,
			"error", err,
		)
		return
	}

	delay := q.baseDelay * time.Duration(1<<(it.attempts-1))
	q.logger.Warn("transient delivery failure, scheduling retry",
		"channel_id", it.msg.ChannelID,
		"attempts", it.attempts,
		"next_delay", delay,
		"error", err,
	)

	go func() {
		select {
		case <-time.After(delay):
			select {
			case q.ch <- it:
			default:
				q.logger.Warn("queue full during retry, message dropped",
					"channel_id", it.msg.ChannelID,
				)
			}
		case <-ctx.Done():
		case <-q.done:
		}
	}()
}
