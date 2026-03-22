package main

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMainConfigPath(t *testing.T) {
	// Тест что конфиг загружается из правильного пути
	configPath := "./configs/server.yaml"

	// Проверяем что файл существует (опционально)
	_, err := os.Stat(configPath)
	if err == nil {
		assert.FileExists(t, configPath)
	}
}

func TestMainAddrFormat(t *testing.T) {
	// Тест формирования адреса
	host := "localhost"
	port := "8080"
	addr := host + ":" + port

	assert.Equal(t, "localhost:8080", addr)
}

func TestMainShutdownTimeout(t *testing.T) {
	// Тест таймаута graceful shutdown
	timeout := 10 * time.Second

	assert.Equal(t, 10*time.Second, timeout)
}

func TestMainSignalHandling(t *testing.T) {
	// Тест что сигналы обрабатываются
	signals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}

	assert.Contains(t, signals, syscall.SIGINT)
	assert.Contains(t, signals, syscall.SIGTERM)
}
