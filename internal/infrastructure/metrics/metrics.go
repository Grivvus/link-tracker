package metrics

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type Metrics struct {
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	GRPCRequestsTotal   *prometheus.CounterVec
	GRPCRequestDuration *prometheus.HistogramVec
}

func New(serviceName string) *Metrics {
	return &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: serviceName,
				Subsystem: "http",
				Name:      "requests_total",
			},
			[]string{"method", "route", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: serviceName,
				Subsystem: "http",
				Name:      "request_duration_s",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "route", "status"},
		),
		GRPCRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: serviceName,
				Subsystem: "grpc",
				Name:      "requests_total",
			},
			[]string{"method", "code"},
		),
		GRPCRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: serviceName,
				Subsystem: "grpc",
				Name:      "request_duration_s",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "code"},
		),
	}
}

func (m *Metrics) Register(registry *prometheus.Registry) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("can't register some of the collectors: %v", r)
		}
	}()
	registry.MustRegister(
		m.HTTPRequestsTotal, m.HTTPRequestDuration,
		m.GRPCRequestsTotal, m.GRPCRequestDuration,
	)

	return err
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sr := &statusRecorder{
			w, http.StatusOK,
		}

		next.ServeHTTP(sr, r)

		route := r.Pattern
		if route == "" {
			route = "unknown"
		}

		status := strconv.Itoa(sr.statusCode)

		duration := time.Since(start).Seconds()

		m.HTTPRequestDuration.WithLabelValues(r.Method, route, status).Observe(duration)
		m.HTTPRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
	})
}

func (m *Metrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		duration := time.Since(start).Seconds()
		code := strconv.FormatUint(uint64(status.Code(err)), 10)

		m.GRPCRequestDuration.WithLabelValues(info.FullMethod, code).Observe(duration)
		m.GRPCRequestsTotal.WithLabelValues(info.FullMethod, code).Inc()

		return resp, err
	}
}

type statusRecorder struct {
	// embed standard response writer
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}
