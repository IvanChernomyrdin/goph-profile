package resilience

import (
	"errors"
	"log/slog"
	"time"

	"github.com/sony/gobreaker"
)

func New(name string, logger *slog.Logger) *gobreaker.CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,
		Interval:    30 * time.Second,
		Timeout:     15 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			if logger != nil {
				logger.Warn("circuit breaker state changed",
					"name", name,
					"from", from.String(),
					"to", to.String(),
				)
			}
		},
		IsSuccessful: func(err error) bool {
			return err == nil
		},
	}

	return gobreaker.NewCircuitBreaker(settings)
}

func IsOpen(err error) bool {
	return errors.Is(err, gobreaker.ErrOpenState)
}
