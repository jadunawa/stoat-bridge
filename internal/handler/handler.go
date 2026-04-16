package handler

import (
	"net/http"

	"github.com/jadunawa/stoat-bridge/internal/message"
)

// Handler parses a webhook request into normalized messages.
type Handler interface {
	Name() string
	Parse(r *http.Request) ([]message.Message, error)
}

// Options holds shared configuration for all handlers.
type Options struct {
	CriticalChannelID string
	WarningChannelID  string
	DefaultChannelID  string
	Template          string
	MaxMessageLength  int
}

// Registry maps handler names to Handler implementations.
type Registry struct {
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

func (r *Registry) Register(h Handler) {
	r.handlers[h.Name()] = h
}

func (r *Registry) Get(name string) (Handler, bool) {
	h, ok := r.handlers[name]
	return h, ok
}

func (r *Registry) Handlers() map[string]Handler {
	result := make(map[string]Handler, len(r.handlers))
	for k, v := range r.handlers {
		result[k] = v
	}
	return result
}
