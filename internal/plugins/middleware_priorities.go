package plugins

// Core middleware priority suggestions. Plugin middleware should pick values
// relative to these to ensure predictable order.
const (
	PriorityRecovery       = 0
	PriorityLogging        = 10
	PriorityRequestID      = 20
	PriorityTracingMetrics = 30
	PriorityCORS           = 40
	PriorityAuth           = 50
	PriorityChannel        = 60
)
