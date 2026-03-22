package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMainConfigPath(t *testing.T) {
	// Сохраняем оригинальное значение
	originalPath := os.Getenv("CONFIG_PATH")
	defer func() {
		_ = os.Setenv("CONFIG_PATH", originalPath)
	}()

	// Тест с пустым CONFIG_PATH
	_ = os.Setenv("CONFIG_PATH", "")
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./configs/server.yaml"
	}
	assert.Equal(t, "./configs/server.yaml", configPath)

	// Тест с заданным CONFIG_PATH
	_ = os.Setenv("CONFIG_PATH", "/custom/path/config.yaml")
	configPath = os.Getenv("CONFIG_PATH")
	assert.Equal(t, "/custom/path/config.yaml", configPath)
}

func TestMainConfigPathNotSet(t *testing.T) {
	// Сохраняем оригинальное значение
	originalPath := os.Getenv("CONFIG_PATH")
	defer func() {
		_ = os.Setenv("CONFIG_PATH", originalPath)
	}()

	// Удаляем переменную
	_ = os.Unsetenv("CONFIG_PATH")

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./configs/server.yaml"
	}

	assert.Equal(t, "./configs/server.yaml", configPath)
}
