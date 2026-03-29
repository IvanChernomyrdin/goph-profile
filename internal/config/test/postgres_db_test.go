package test

import (
	"goph-profile-avatars/internal/config"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostgresInit(t *testing.T) {
	// Проверяем, что переменная окружения с тестовым DSN есть
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set, skipping integration test")
	}

	// Инициализация базы данных
	err := config.PostgresInit(dsn)
	require.NoError(t, err, "PostgresInit should not return an error")

	db := config.GetDB()
	require.NotNil(t, db, "DB should be initialized")

	// Проверяем, что соединение работает
	err = db.Ping()
	require.NoError(t, err, "DB Ping should succeed")

	// Дополнительно: можно проверить, что глобальная переменная DB совпадает с возвращаемой
	require.Equal(t, db, config.GetDB())
}
