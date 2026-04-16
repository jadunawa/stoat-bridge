package queue

// MetricsReporter is the callback interface the queue uses to report delivery metrics.
type MetricsReporter interface {
	RecordDelivered(channelID string)
	RecordDropped(reason string)
	ObserveDeliveryDuration(seconds float64)
	SetQueueDepth(depth float64)
}

type noopReporter struct{}

func (noopReporter) RecordDelivered(string)         {}
func (noopReporter) RecordDropped(string)            {}
func (noopReporter) ObserveDeliveryDuration(float64) {}
func (noopReporter) SetQueueDepth(float64)           {}
