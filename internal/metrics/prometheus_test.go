package metrics

import (
	"testing"
)

func TestPrometheusObserver(t *testing.T) {
	obs := NewPrometheusObserver()

	// Just call methods to ensure no panic
	obs.IncOnline()
	obs.DecOnline()
	obs.RecordPush()
}
