// Package config содержит инициализацию подключения к базе данных сервера
// и доступ к глобальному экземпляру *sql.DB.
//
// Пакет выполняет:
//   - открытие соединения с PostgreSQL (через драйвер pgx);
//   - проверку доступности базы (Ping);
//   - запуск миграций (golang-migrate) при старте сервера.
//
// Примечание: пакет использует глобальную переменную DB. Инициализация должна
// выполняться один раз при запуске сервера.
package config

import (
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v4/stdlib"
)

// DB — глобальный экземпляр подключения к базе данных.
//
// Инициализируется функцией PostgresInit и используется другими пакетами через GetDB.
var DB *sql.DB

// PostgresInit открывает подключение к базе данных по DSN, проверяет его доступность
// и применяет миграции.
//
// databaseDSN — строка подключения к PostgreSQL.
// Миграции запускаются из каталога file://migrations/postgres.
// Если миграции уже применены, ошибка migrate.ErrNoChange не считается ошибкой.
func PostgresInit(databaseDSN string) error {
	logger := slog.Default().With(
		"component", "postgres-init",
	)

	var err error
	DB, err = sql.Open("pgx", databaseDSN)
	if err != nil {
		logger.Error("failed to open db connection", "error", err)
		return err
	}

	if err = DB.Ping(); err != nil {
		logger.Error("failed to ping db", "error", err)
		return err
	}

	migrationsPath := "migrations/postgres"

	// если папки миграций ещё нет — просто пропускаем
	if _, err := os.Stat(migrationsPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Warn("migrations directory not found, skipping migrations", "path", migrationsPath)
			return nil
		}
		logger.Error("failed to check migrations directory", "error", err)
		return err
	}

	// если папка есть, но в ней нет migration-файлов — тоже пропускаем
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		logger.Error("failed to read migrations directory", "error", err)
		return err
	}

	hasMigrationFiles := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") || strings.HasSuffix(name, ".down.sql") {
			hasMigrationFiles = true
			break
		}
	}

	if !hasMigrationFiles {
		logger.Warn("no migration files found, skipping migrations", "path", migrationsPath)
		return nil
	}

	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		logger.Error("failed to resolve migrations path", "error", err)
		return err
	}

	absPath = filepath.ToSlash(absPath)

	// Запуск миграций
	driver, err := postgres.WithInstance(DB, &postgres.Config{})
	if err != nil {
		logger.Error("failed to create migration driver", "error", err)
		return err
	}

	// создаём миграции с выбранным драйвером
	m, err := migrate.NewWithDatabaseInstance(
		"file://"+absPath,
		"postgres",
		driver,
	)
	if err != nil {
		logger.Error("failed to create migrations instance", "error", err)
		return err
	}

	// запускаем миграции
	err = m.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("no new migrations to apply")
			return nil
		}

		logger.Error("failed to apply migrations", "error", err)
		return err
	}

	logger.Info("migrations applied successfully")
	return nil
}

// GetDB возвращает текущий глобальный экземпляр *sql.DB.
//
// Возвращаемое значение может быть nil, если PostgresInit ещё не вызывался
// или завершился ошибкой.
func GetDB() *sql.DB {
	return DB
}
