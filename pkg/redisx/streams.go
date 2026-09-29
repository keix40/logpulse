package redisx

const (
	StreamLogs       = "logpulse:logs"
	StreamAlerterDLQ = "logpulse:alerter-dlq"
	ChannelLive      = "logpulse:live"
	FieldPayload     = "payload"
	FieldError       = "error"
	FieldSourceID    = "source_id"
	ConsumerWorker   = "worker"
	ConsumerAlerter  = "alerter"
	GroupWorker      = "logpulse-workers"
	GroupAlerter     = "logpulse-alerter"
)
