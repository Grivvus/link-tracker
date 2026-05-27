package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
)

type ScrapperMetrics struct {
	RED                    *Metrics
	LinkOnTrackTotal       *prometheus.GaugeVec
	RequestDurationMSTotal *prometheus.HistogramVec
	APIRequestsTotal       *prometheus.CounterVec
	repo                   repository.LinksMetricsRepository
}

func NewScrapperMetrics(repo repository.LinksMetricsRepository) *ScrapperMetrics {
	const namespace = "scrapper"
	return &ScrapperMetrics{
		repo: repo,
		RED:  New(namespace),
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

func (m *ScrapperMetrics) LinksOnTrackCntJob(
	ctx context.Context, logger *slog.Logger, period time.Duration,
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			gh, st, err := m.repo.TrackedLinksCount(ctx)
			if err != nil {
				logger.Error("can't get LinksOnTrack metric", "err", err)
			} else {
				m.LinkOnTrackTotal.WithLabelValues("github").Set(float64(gh))
				m.LinkOnTrackTotal.WithLabelValues("stackoverflow").Set(float64(st))
			}
		}

		time.Sleep(period)
	}
}
