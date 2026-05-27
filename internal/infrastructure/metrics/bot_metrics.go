package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type BotMetrics struct {
	RED                    *Metrics
	SentNotificationsTotal prometheus.Counter
	CommandRequestsTotatl  *prometheus.CounterVec
	CommandDurationMSTotal *prometheus.HistogramVec
}

func NewBotMetrics() *BotMetrics {
	const namespace = "bot"
	return &BotMetrics{
		RED: New(namespace),
		SentNotificationsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sent_notification_total",
		}),
		CommandRequestsTotatl: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "command_requests_total",
			},
			[]string{"command"},
		),
		CommandDurationMSTotal: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "command_duration_ms_total",
			},
			[]string{"scope", "scope_type"},
		),
	}
}

func (m *BotMetrics) Register(registry *prometheus.Registry) error {
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

	registry.MustRegister(
		m.CommandDurationMSTotal,
		m.CommandRequestsTotatl,
		m.SentNotificationsTotal,
	)

	return err
}
