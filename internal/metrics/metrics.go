package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "route", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route", "status"},
	)

	UploadsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_uploads_total",
			Help: "Total number of avatar uploads",
		},
		[]string{"status"},
	)

	UploadDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "avatars_upload_duration_seconds",
			Help:    "Avatar upload duration",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
		},
		[]string{"status"},
	)

	DownloadsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_downloads_total",
			Help: "Total number of avatar downloads",
		},
		[]string{"status"},
	)

	DownloadDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "avatars_download_duration_seconds",
			Help:    "Avatar download duration",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"status"},
	)

	DeletesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_deletes_total",
			Help: "Total number of avatar deletions",
		},
		[]string{"status"},
	)

	RabbitPublishesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_publishes_total",
			Help: "Total number of RabbitMQ publish attempts",
		},
		[]string{"event", "status"},
	)
)
