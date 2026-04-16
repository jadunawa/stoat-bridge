package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jadunawa/stoat-bridge/internal/handler"
	"github.com/jadunawa/stoat-bridge/internal/message"
	"github.com/jadunawa/stoat-bridge/internal/metrics"
)

// Enqueuer is the interface the server uses to submit messages.
type Enqueuer interface {
	Enqueue(msg message.Message) bool
}

// Config holds server-specific configuration.
type Config struct {
	MaxBodySize   int64
	RateLimit     int
	WebhookSecret string
}

// Server is the HTTP server that handles webhook ingestion.
type Server struct {
	router   chi.Router
	registry *handler.Registry
	queue    Enqueuer
	metrics  *metrics.Metrics
	cfg      *Config
	logger   *slog.Logger
	ready    atomic.Bool
}

// New creates a new Server.
func New(cfg *Config, reg *handler.Registry, q Enqueuer, met *metrics.Metrics) *Server {
	s := &Server{
		registry: reg,
		queue:    q,
		metrics:  met,
		cfg:      cfg,
		logger:   slog.Default(),
	}
	s.buildRouter()
	return s
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	return s.router
}

// SetReady sets the readiness state.
func (s *Server) SetReady(ready bool) {
	s.ready.Store(ready)
}

func (s *Server) buildRouter() {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.loggerMiddleware)

	if s.cfg.RateLimit > 0 {
		r.Use(httprate.LimitAll(s.cfg.RateLimit, 1*time.Second))
	}

	// Operational endpoints (no auth)
	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)
	r.Get("/metrics", s.handleMetrics)

	// Webhook endpoints (with auth if configured)
	r.Group(func(r chi.Router) {
		if s.cfg.WebhookSecret != "" {
			r.Use(s.authMiddleware)
		}
		r.Use(s.bodySizeMiddleware)

		// Auto-detect endpoint
		r.Post("/webhook", s.handleWebhook)

		// Named endpoints — one per registered handler
		for name := range s.registry.Handlers() {
			handlerName := name // capture for closure
			r.Post("/"+handlerName, s.handleNamed(handlerName))
		}
	})

	s.router = r
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `{"status":"ok"}`)
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, `{"status":"not ready"}`)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `{"status":"ok"}`)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(s.metrics.Registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

func (s *Server) handleNamed(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h, ok := s.registry.Get(name)
		if !ok {
			http.Error(w, "handler not found", http.StatusInternalServerError)
			return
		}
		s.processWebhook(w, r, h, name)
	}
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.metrics.WebhooksReceived.WithLabelValues("unknown", "400").Inc()
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	name, err := handler.Detect(body)
	if err != nil {
		s.metrics.WebhooksReceived.WithLabelValues("unknown", "400").Inc()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h, ok := s.registry.Get(name)
	if !ok {
		s.metrics.WebhooksReceived.WithLabelValues(name, "500").Inc()
		http.Error(w, "detected handler not registered", http.StatusInternalServerError)
		return
	}

	// Replace the body so the handler can read it
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	s.processWebhook(w, r, h, name)
}

func (s *Server) processWebhook(w http.ResponseWriter, r *http.Request, h handler.Handler, source string) {
	reqID := middleware.GetReqID(r.Context())
	logger := s.logger.With("request_id", reqID, "source", source)

	msgs, err := h.Parse(r)
	if err != nil {
		s.metrics.WebhooksReceived.WithLabelValues(source, "400").Inc()
		logger.Warn("webhook parse error", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, msg := range msgs {
		if s.queue.Enqueue(msg) {
			s.metrics.MessagesQueued.Inc()
			logger.Debug("message enqueued", "channel_id", msg.ChannelID)
		} else {
			s.metrics.MessagesDropped.WithLabelValues("buffer_full").Inc()
			logger.Warn("message dropped, queue full", "channel_id", msg.ChannelID)
		}
	}

	s.metrics.WebhooksReceived.WithLabelValues(source, "202").Inc()
	w.WriteHeader(http.StatusAccepted)
	_, _ = fmt.Fprintf(w, `{"status":"accepted","messages":%d}`, len(msgs))
}

// Middleware

func (s *Server) loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())
		s.logger.Debug("request received",
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + s.cfg.WebhookSecret

		if auth != expected {
			s.logger.Warn("unauthorized request",
				"request_id", middleware.GetReqID(r.Context()),
				"remote_addr", r.RemoteAddr,
			)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) bodySizeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodySize)
		next.ServeHTTP(w, r)
	})
}
