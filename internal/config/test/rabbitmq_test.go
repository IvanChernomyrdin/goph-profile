package test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"goph-profile-avatars/internal/config"
)

func TestRabbitMQInit(t *testing.T) {
	url := os.Getenv("TEST_RABBITMQ_URL")
	if url == "" {
		t.Skip("TEST_RABBITMQ_URL not set, skipping integration test")
	}

	cfg := config.RabbitMQConfig{
		URL:              url,
		Exchange:         "test.exchange",
		ExchangeType:     "direct",
		QueueUpload:      "test.upload.queue",
		QueueDelete:      "test.delete.queue",
		UploadRoutingKey: "test.uploaded",
		DeleteRoutingKey: "test.deleted",
	}

	// Инициализация RabbitMQ
	err := config.RabbitMQInit(cfg)
	require.NoError(t, err, "RabbitMQInit should not return an error")

	conn := config.GetRabbitConn()
	ch := config.GetRabbitChannel()
	require.NotNil(t, conn, "RabbitMQ connection should be initialized")
	require.NotNil(t, ch, "RabbitMQ channel should be initialized")

	// Проверка, что соединение открыто
	ch2, err := conn.Channel()
	require.NoError(t, err, "Connection should be usable")
	defer func() {
		_ = ch2.Close() // закрываем временный канал
	}()

	require.NoError(t, err, "Connection should be usable")

	// Закрываем ресурсы
	err = config.CloseRabbitMQ()
	require.NoError(t, err, "CloseRabbitMQ should not return an error")
}
