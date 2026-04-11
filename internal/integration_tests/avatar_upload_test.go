package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"goph-profile-avatars/internal/api"
	apphttp "goph-profile-avatars/internal/net/http"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"

	status "goph-profile-avatars/internal/config/status"

	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	defaultPostgresDSN   = "postgres://postgres:postgres@127.0.0.1:5432/gophprofile?sslmode=disable"
	defaultMinioEndpoint = "127.0.0.1:9000"
	defaultBucket        = "avatars"
	defaultRabbitURL     = "amqp://guest:guest@127.0.0.1:5672/"
)

type uploadResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	URL       string `json:"url"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type avatarRow struct {
	ID               string
	UserID           string
	FileName         string
	MimeType         string
	SizeBytes        int64
	S3Key            string
	ThumbnailS3Keys  []byte
	UploadStatus     string
	ProcessingStatus string
}

func TestUploadAvatar_Integration(t *testing.T) {
	db := mustOpenDB(t)
	go func() {
		_ = db.Close()
	}()

	minioClient := mustNewMinio(t)
	cleanupState(t, db, minioClient)

	ts := newTestHTTPServer(t, db)
	defer ts.Close()

	respBody := uploadAvatarToURL(t, ts.URL, "user-123")

	if respBody.ID == "" {
		t.Fatal("expected avatar id")
	}
	if respBody.UserID != "user-123" {
		t.Fatalf("unexpected user id: %s", respBody.UserID)
	}
	if respBody.Status != "processing" {
		t.Fatalf("unexpected status: %s", respBody.Status)
	}

	row := waitForAvatarCreated(t, db, respBody.ID, 5*time.Second)

	if row.UserID != "user-123" {
		t.Fatalf("unexpected db user_id: %s", row.UserID)
	}
	if row.UploadStatus != status.Uploaded {
		t.Fatalf("unexpected upload_status: %s", row.UploadStatus)
	}
	if row.ProcessingStatus != status.Pending {
		t.Fatalf("unexpected processing_status: %s", row.ProcessingStatus)
	}
	if row.S3Key == "" {
		t.Fatal("expected s3_key")
	}

	_, err := minioClient.StatObject(context.Background(), bucketName(), row.S3Key, minio.StatObjectOptions{})
	if err != nil {
		t.Fatalf("original file not found in minio: %v", err)
	}
}

func TestUploadAvatar_InvalidMime(t *testing.T) {
	db := mustOpenDB(t)
	go func() {
		_ = db.Close()
	}()

	minioClient := mustNewMinio(t)
	cleanupState(t, db, minioClient)

	ts := newTestHTTPServer(t, db)
	defer ts.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "file.txt")
	if err != nil {
		t.Fatal(err)
	}

	_, _ = part.Write([]byte("hello world"))

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/avatars", &body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "user-123")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM public.avatars`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("expected 0 avatars, got %d", count)
	}
}

func newTestHTTPServer(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()

	minioClient := mustNewMinio(t)

	amqpConn, err := amqp.Dial(rabbitURL())
	if err != nil {
		t.Fatalf("amqp dial: %v", err)
	}
	t.Cleanup(func() { _ = amqpConn.Close() })

	amqpCh, err := amqpConn.Channel()
	if err != nil {
		t.Fatalf("amqp channel: %v", err)
	}
	t.Cleanup(func() { _ = amqpCh.Close() })

	if err := amqpCh.ExchangeDeclare("avatars.exchange", "direct", true, false, false, false, nil); err != nil {
		t.Fatalf("exchange declare: %v", err)
	}

	repo := repository.NewAvatarRepository(db)
	storage := services.NewMinIOStorage(minioClient, bucketName(), &slog.Logger{})
	publisher := services.NewRabbitPublisher(amqpCh, "avatars.exchange", "avatar.uploaded", "avatar.deleted", &slog.Logger{})
	avatarService := services.NewAvatarService(repo, storage, publisher)

	healthService := &api.HealthService{}
	handler := api.NewHandler(healthService, avatarService, &atomic.Bool{})

	router := apphttp.NewRouter(handler, 1000)
	return httptest.NewServer(router)
}

func mustOpenDB(t *testing.T) *sql.DB {
	t.Helper()

	_ = os.Unsetenv("PGLOCALEDIR")

	db, err := sql.Open("postgres", postgresDSN())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	return db
}

func mustNewMinio(t *testing.T) *minio.Client {
	t.Helper()

	client, err := minio.New(minioEndpoint(), &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("create minio client: %v", err)
	}

	return client
}

func cleanupState(t *testing.T, db *sql.DB, minioClient *minio.Client) {
	t.Helper()

	_, err := db.Exec(`DELETE FROM public.avatars`)
	if err != nil {
		t.Fatalf("cleanup db: %v", err)
	}

	ctx := context.Background()
	for object := range minioClient.ListObjects(ctx, bucketName(), minio.ListObjectsOptions{
		Recursive: true,
	}) {
		if object.Err != nil {
			t.Fatalf("list objects: %v", object.Err)
		}
		if err := minioClient.RemoveObject(ctx, bucketName(), object.Key, minio.RemoveObjectOptions{}); err != nil {
			t.Fatalf("remove object %s: %v", object.Key, err)
		}
	}
}

func uploadAvatarToURL(t *testing.T, serverURL, userID string) uploadResponse {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "avatar.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	img := validPNG(t)
	if _, err := part.Write(img); err != nil {
		t.Fatalf("write image: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, serverURL+"/api/v1/avatars", &body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", userID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	go func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	var result uploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return result
}

func waitForAvatarCreated(t *testing.T, db *sql.DB, avatarID string, timeout time.Duration) avatarRow {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		var row avatarRow

		err := db.QueryRow(`
			SELECT
				id,
				user_id,
				file_name,
				mime_type,
				size_bytes,
				s3_key,
				thumbnail_s3_keys,
				upload_status,
				processing_status
			FROM public.avatars
			WHERE id = $1
		`, avatarID).Scan(
			&row.ID,
			&row.UserID,
			&row.FileName,
			&row.MimeType,
			&row.SizeBytes,
			&row.S3Key,
			&row.ThumbnailS3Keys,
			&row.UploadStatus,
			&row.ProcessingStatus,
		)
		if err == nil {
			return row
		}

		if err != sql.ErrNoRows {
			t.Fatalf("select avatar while waiting: %v", err)
		}

		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("avatar %s was not created in time", avatarID)
	return avatarRow{}
}

func validPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 7),
				G: uint8(y * 7),
				B: 120,
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return buf.Bytes()
}

func postgresDSN() string {
	if v := os.Getenv("INTEGRATION_PG_DSN"); v != "" {
		return v
	}
	return defaultPostgresDSN
}

func minioEndpoint() string {
	if v := os.Getenv("INTEGRATION_MINIO_ENDPOINT"); v != "" {
		return v
	}
	return defaultMinioEndpoint
}

func bucketName() string {
	if v := os.Getenv("INTEGRATION_MINIO_BUCKET"); v != "" {
		return v
	}
	return defaultBucket
}

func rabbitURL() string {
	if v := os.Getenv("INTEGRATION_RABBIT_URL"); v != "" {
		return v
	}
	return defaultRabbitURL
}
