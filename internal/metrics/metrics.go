package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Метрики загрузки
	UploadsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_uploads_total",
			Help: "Total number of avatar uploads",
		},
		[]string{"status", "user_id"},
	)

	UploadDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "avatars_upload_duration_seconds",
			Help: "Avatar upload duration",
		},
		[]string{"status"},
	)

	// Метрики скачивания
	DownloadsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_downloads_total",
			Help: "Total number of avatar downloads",
		},
		[]string{"status"},
	)

	DownloadDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "avatars_download_duration_seconds",
			Help: "Avatar download duration",
		},
		[]string{"status"},
	)

	// Метрики удаления
	DeletesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_deletes_total",
			Help: "Total number of avatar deletions",
		},
		[]string{"status"},
	)
)
