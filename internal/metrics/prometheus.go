package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type prometheusObserver struct {
	onlineGauge prometheus.Gauge
	pushCounter prometheus.Counter
	pushLatency prometheus.Histogram
	eventLag    prometheus.Gauge
}

var (
	onlineGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mizuflow_online_clients",
		Help: "Number of online MizuFlow clients",
	})
	pushCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mizuflow_push_total",
		Help: "Total number of feature pushes",
	})
	pushLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "mizuflow_push_latency_seconds",
		Help:    "Latency of feature pushes in seconds",
		Buckets: prometheus.DefBuckets,
	})
	eventLag = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mizuflow_event_lag",
		Help: "Lag between event creation and processing",
	})
)

func NewPrometheusObserver() HubObserver {
	return &prometheusObserver{
		onlineGauge: onlineGauge,
		pushCounter: pushCounter,
		pushLatency: pushLatency,
		eventLag:    eventLag,
	}
}

func Handler() http.Handler {
	return promhttp.Handler()
}
func (p *prometheusObserver) IncOnline() {
	p.onlineGauge.Inc()
}
func (p *prometheusObserver) DecOnline() {
	p.onlineGauge.Dec()
}
func (p *prometheusObserver) RecordPush() {
	p.pushCounter.Inc()
}

func (p *prometheusObserver) ObservePushLatency(duration float64) {
	p.pushLatency.Observe(duration)
}

func (p *prometheusObserver) UpdateEventLag(lag int) {
	p.eventLag.Set(float64(lag))
}
