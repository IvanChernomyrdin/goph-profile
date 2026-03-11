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
	"os"
	"path/filepath"
	"strings"

	logger "goph-profile-avatars/internal/logging"

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
	customLog := logger.NewHTTPLogger().Logger.Sugar()

	var err error
	DB, err = sql.Open("pgx", databaseDSN)

	if err != nil {
		customLog.Errorf("error to connect db: %v", err)
		return err
	}

	if err = DB.Ping(); err != nil {
		customLog.Errorf("error check db connection: %v", err)
		return err
	}

	migrationsPath := "migrations/postgres"

	// если папки миграций ещё нет — просто пропускаем
	if _, err := os.Stat(migrationsPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			customLog.Warnf("migrations directory %s not found, skipping migrations", migrationsPath)
			return nil
		}
		customLog.Errorf("error checking migrations directory: %v", err)
		return err
	}

	// если папка есть, но в ней нет migration-файлов — тоже пропускаем
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		customLog.Errorf("error reading migrations directory: %v", err)
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
		customLog.Warnf("no migration files found in %s, skipping migrations", migrationsPath)
		return nil
	}

	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		customLog.Errorf("error resolving migrations path: %v", err)
		return err
	}

	absPath = filepath.ToSlash(absPath)

	// Запуск миграций
	driver, err := postgres.WithInstance(DB, &postgres.Config{})
	if err != nil {
		customLog.Errorf("error creating migration driver: %v", err)
		return err
	}

	// создаём миграции с выбранным драйвером
	m, err := migrate.NewWithDatabaseInstance(
		"file:///"+absPath,
		"postgres", driver)
	if err != nil {
		customLog.Errorf("error creating migrations: %v", err)
		return err
	}

	// запускаем создание миграций
	err = m.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			customLog.Info("no new migrations to apply")
			return nil
		}

		customLog.Errorf("error applying migrations: %v", err)
		return err
	}

	customLog.Info("migrations applied successfully")
	return nil
}

// GetDB возвращает текущий глобальный экземпляр *sql.DB.
//
// Возвращаемое значение может быть nil, если PostgresInit ещё не вызывался
// или завершился ошибкой.
func GetDB() *sql.DB {
	return DB
}
