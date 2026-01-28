package metrics

type HubObserver interface {
	IncOnline()
	DecOnline()
	RecordPush()
}
