package infrastructure

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/sony/gobreaker/v2"
)

func KafkaLoggerFn(sloggerFn func(string, ...any)) func(string, ...any) {
	return func(format string, v ...any) {
		sloggerFn(fmt.Sprintf(format, v...))
	}
}

func NewBreaker(
	logger *slog.Logger, rcfg ReliabiltyConfig,
	breakerName string, IsSuccessfulFunc func(error) bool,
) *gobreaker.TwoStepCircuitBreaker[struct{}] {
	breaker := gobreaker.NewTwoStepCircuitBreaker[struct{}](gobreaker.Settings{
		Name:         breakerName,
		MaxRequests:  rcfg.BreakerHalfopenMaxRequests,
		Interval:     time.Duration(rcfg.BreakerIntervalSecond) * time.Second,
		Timeout:      time.Duration(rcfg.BreakerTimeoutSecond) * time.Second,
		BucketPeriod: time.Duration(rcfg.BreakerBucketPeriodSecond) * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests == 0 {
				return false
			}
			failurePercentage := float64(counts.TotalFailures) / float64(counts.Requests) * 100
			return failurePercentage >= float64(rcfg.BreakerFailureTreshold)
		},
		IsSuccessful: func(err error) bool {
			return err == nil
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logger.Warn("circuit breaker state changed", "name", name, "from", from, "to", to)
		},
	})
	return breaker
}

func ExecuteWithBreaker[T any](
	cb *gobreaker.TwoStepCircuitBreaker[struct{}],
	op func() (T, error),
) (T, error) {
	done, err := cb.Allow()
	if err != nil {
		var zero T
		return zero, err
	}

	result, err := op()
	done(err)

	return result, err
}
