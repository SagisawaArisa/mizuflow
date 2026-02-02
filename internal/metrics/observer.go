package metrics

type HubObserver interface {
	IncOnline()
	DecOnline()
	RecordPush()
	ObservePushLatency(duration float64)
	UpdateEventLag(lag int)
}
