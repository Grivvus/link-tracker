package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type ScrapperMetrics struct {
	RED                    *Metrics
	LinkOnTrackTotal       *prometheus.GaugeVec
	RequestDurationMSTotal *prometheus.HistogramVec
	APIRequestsTotal       *prometheus.CounterVec
}

func NewScrapperMetrics() *ScrapperMetrics {
	const namespace = "scrapper"
	return &ScrapperMetrics{
		RED: New(namespace),
		LinkOnTrackTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "links_on_track_total",
			},
			[]string{"tracked_source"},
		),
		RequestDurationMSTotal: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "request_duration_ms_total",
			},
			[]string{"scope", "scope_type"},
		),
		APIRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "api_requests_total",
			},
			[]string{"source"},
		),
	}
}

func (m *ScrapperMetrics) Register(registry *prometheus.Registry) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("can't register some of the collectors: %v", r)
		}
	}()

	err = m.RED.Register(registry)
	if err != nil {
		return err
	}

	return err
}
