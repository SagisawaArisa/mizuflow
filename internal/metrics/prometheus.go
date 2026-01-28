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
)

func NewPrometheusObserver() HubObserver {
	return &prometheusObserver{
		onlineGauge: onlineGauge,
		pushCounter: pushCounter,
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
