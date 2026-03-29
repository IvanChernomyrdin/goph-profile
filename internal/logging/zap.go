// Package logger содержит общий логгер для server и agent.
//
// Пакет предоставляет Zap-логгер, настроенный на запись в файл с ротацией
// (lumberjack) и удобный метод для логирования HTTP-запросов.
package logger

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// HTTPLogger представляет обёртку над zap.Logger для логирования HTTP-событий.
//
// Встраивание *zap.Logger позволяет использовать все методы zap напрямую.
type HTTPLogger struct {
	*zap.Logger
}

// NewHTTPLogger создаёт файловый zap-логгер для HTTP-логов.
//
// Логи записываются в файл runtime/logs/http.log.
// Для файлов включена ротация (MaxSize/MaxBackups/MaxAge) и сжатие архивов.
// Формат времени: "HH:MM:SS DD.MM.YYYY".
func NewHTTPLogger() *HTTPLogger {
	logDir := filepath.Join("runtime", "logs")
	_ = os.MkdirAll(logDir, 0755)

	logFile := filepath.Join(logDir, "http.log")

	// lumberjack отвечает за ротацию файлов
	writer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    100, // MB ≈ ~300 000 строк
		MaxBackups: 10,  // сколько старых файлов хранить
		MaxAge:     30,  // дней
		Compress:   true,
	})

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = customTimeEncoder

	// выводим обычный текст, раньше я выводи в json
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg),
		writer,
		zap.InfoLevel,
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &HTTPLogger{Logger: logger}
}

// LogRequest записывает структурированный лог об HTTP-запросе.
//
// method и uri — параметры запроса,
// status — HTTP-статус ответа,
// responseSize — размер ответа в байтах,
// duration — длительность обработки запроса в миллисекундах.
func (logger *HTTPLogger) LogRequest(method, uri string, status, responseSize int, duration float64) {
	logger.Info("HTTP request",
		zap.String("method", method),
		zap.String("uri", uri),
		zap.Int("status", status),
		zap.Int("response_size", responseSize),
		zap.Float64("duration_ms", duration),
	)
}

// customTimeEncoder форматирует время для логов в виде "HH:MM:SS DD.MM.YYYY".
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("15:04:05 02.01.2006"))
}
