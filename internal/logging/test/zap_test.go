package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	logger "goph-profile-avatars/internal/logging"
)

// helper: создаём временный HTTPLogger для теста
func newTestHTTPLogger(t *testing.T) (*logger.HTTPLogger, string) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "httplog_*.log")
	if err != nil {
		t.Fatalf("failed to create temp log file: %v", err)
	}

	defer func() {
		_ = tmpFile.Close()
	}()

	// временно переопределяем путь лог-файла внутри NewHTTPLogger
	oldPath := filepath.Join("runtime", "logs", "http.log")
	defer func() {
		_ = os.Remove(oldPath) // очистка старого пути
	}()

	l := logger.NewHTTPLogger()
	return l, oldPath
}

func TestNewHTTPLogger_CreatesLogFileAndWrites(t *testing.T) {
	l, file := newTestHTTPLogger(t)

	l.Info("test message")
	_ = l.Sync()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	s := string(data)

	if !regexp.MustCompile(`\btest message\b`).MatchString(s) {
		t.Fatalf("expected log to contain message, got: %q", s)
	}

	timeRe := regexp.MustCompile(`\b\d{2}:\d{2}:\d{2} \d{2}\.\d{2}\.\d{4}\b`)
	if !timeRe.MatchString(s) {
		t.Fatalf("expected custom time format, got: %q", s)
	}

	_ = os.Remove(file)
}

func TestHTTPLogger_LogRequest_WritesStructuredFields(t *testing.T) {
	l, file := newTestHTTPLogger(t)

	l.LogRequest("POST", "/auth/login", 401, 20, 158.5463)
	_ = l.Sync()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	s := string(data)

	mustContain := []string{
		"HTTP request",
		"method", "POST",
		"uri", "/auth/login",
		"status", "401",
		"response_size", "20",
		"duration_ms",
	}
	for _, sub := range mustContain {
		if !regexp.MustCompile(regexp.QuoteMeta(sub)).MatchString(s) {
			t.Fatalf("expected log to contain %q, got: %q", sub, s)
		}
	}

	_ = os.Remove(file)
}
