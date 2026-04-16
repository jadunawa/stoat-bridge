package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for stoat-bridge.
type Metrics struct {
	WebhooksReceived   *prometheus.CounterVec
	MessagesQueued     prometheus.Counter
	MessagesDelivered  *prometheus.CounterVec
	MessagesDropped    *prometheus.CounterVec
	DeliveryDuration   prometheus.Histogram
	QueueDepth         prometheus.Gauge

	Registry *prometheus.Registry
}

// New creates a new Metrics instance with a dedicated Prometheus registry.
func New() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		WebhooksReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "stoatbridge_webhooks_received_total",
			Help: "Total webhooks received by source and response code.",
		}, []string{"source", "status_code"}),

		MessagesQueued: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "stoatbridge_messages_queued_total",
			Help: "Total messages enqueued for delivery.",
		}),

		MessagesDelivered: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "stoatbridge_messages_delivered_total",
			Help: "Total messages successfully delivered.",
		}, []string{"channel"}),

		MessagesDropped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "stoatbridge_messages_dropped_total",
			Help: "Total messages dropped.",
		}, []string{"reason"}),

		DeliveryDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "stoatbridge_delivery_duration_seconds",
			Help:    "Stoat API call latency.",
			Buckets: prometheus.DefBuckets,
		}),

		QueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "stoatbridge_queue_depth",
			Help: "Current messages waiting in buffer.",
		}),

		Registry: reg,
	}

	reg.MustRegister(
		m.WebhooksReceived,
		m.MessagesQueued,
		m.MessagesDelivered,
		m.MessagesDropped,
		m.DeliveryDuration,
		m.QueueDepth,
	)

	return m
}

func (m *Metrics) RecordDelivered(channelID string) {
	m.MessagesDelivered.WithLabelValues(channelID).Inc()
}

func (m *Metrics) RecordDropped(reason string) {
	m.MessagesDropped.WithLabelValues(reason).Inc()
}

func (m *Metrics) ObserveDeliveryDuration(seconds float64) {
	m.DeliveryDuration.Observe(seconds)
}

func (m *Metrics) SetQueueDepth(depth float64) {
	m.QueueDepth.Set(depth)
}
