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

	// rabbit
	RabbitPublishesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_publishes_total",
			Help: "Total number of RabbitMQ publish attempts",
		},
		[]string{"event", "status"},
	)
	//rabbit Ack/Nack
	RabbitMQMessageAcks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_message_acks_total",
			Help: "Total number of successful message acks",
		},
		[]string{"queue"},
	)

	RabbitMQMessageNacks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_message_nacks_total",
			Help: "Total number of successful message nacks",
		},
		[]string{"queue"},
	)

	RabbitMQMessageAckFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_message_ack_failures_total",
			Help: "Total number of message ack failures",
		},
		[]string{"queue"},
	)

	RabbitMQMessageNackFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_message_nack_failures_total",
			Help: "Total number of message nack failures",
		},
		[]string{"queue"},
	)

	// worker
	WorkerJobsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_worker_jobs_total",
			Help: "Total number of avatar worker jobs",
		},
		[]string{"type", "status"},
	)

	WorkerJobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "avatars_worker_job_duration_seconds",
			Help:    "Worker job duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 30},
		},
		[]string{"type", "status"},
	)

	WorkerProcessingStageDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "avatars_worker_stage_duration_seconds",
			Help:    "Duration of worker processing stages",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2},
		},
		[]string{"stage"},
	)

	WorkerFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_worker_failures_total",
			Help: "Total number of worker failures",
		},
		[]string{"stage"},
	)
)
