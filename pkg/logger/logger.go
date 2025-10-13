package logger

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	mu           sync.RWMutex
)

func init() { // ensure we always have a usable logger even before Init is called
	globalLogger = zap.NewNop()
}

// Init configures the global logger using the provided level string.
func Init(level string) error {
	cfg := zap.NewProductionConfig()

	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)

	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	globalLogger = logger
	return nil
}

// Logger returns the configured global logger.
func Logger() *zap.Logger {
	mu.RLock()
	defer mu.RUnlock()

	return globalLogger
}

// Sync flushes buffered log entries.
func Sync() error {
	return Logger().Sync()
}

// WithModule returns a child logger annotated with the module name.
func WithModule(module string) *zap.Logger {
	return Logger().With(zap.String("module", module))
}

// Info logs an informational message using the global logger.
func Info(msg string, fields ...zap.Field) {
	Logger().Info(msg, fields...)
}

// Error logs an error message using the global logger.
func Error(msg string, fields ...zap.Field) {
	Logger().Error(msg, fields...)
}

// Warn logs a warning message using the global logger.
func Warn(msg string, fields ...zap.Field) {
	Logger().Warn(msg, fields...)
}

// Debug logs a debug message using the global logger.
func Debug(msg string, fields ...zap.Field) {
	Logger().Debug(msg, fields...)
}
