package tornago

import (
	"log/slog"
)

// Logger defines a minimal structured logging interface for tornago.
// This interface is intentionally simple to support various logging libraries
// (slog, logrus, zap, zerolog, etc.) through adapters.
//
// The default logger discards all log messages. Users can provide their own
// logger implementation using WithLogger configuration option.
//
// Example with slog:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
//	    Level: slog.LevelDebug,
//	}))
//	adapter := tornago.NewSlogAdapter(logger)
//	cfg, _ := tornago.NewClientConfig(
//	    tornago.WithClientSocksAddr("127.0.0.1:9050"),
//	    tornago.WithLogger(adapter),
//	)
//
// For other logging libraries, implement this interface by wrapping your
// preferred logger. The keysAndValues parameter should be interpreted as
// alternating key-value pairs (e.g., "key1", value1, "key2", value2).
type Logger interface {
	// Log logs a message at the specified level with optional key-value pairs.
	// Level should be one of: "debug", "info", "warn", "error".
	// The keysAndValues are interpreted as alternating key-value pairs.
	Log(level string, msg string, keysAndValues ...any)
}

// noopLogger is a logger that discards all messages.
type noopLogger struct{}

func (noopLogger) Log(string, string, ...any) {}

// slogAdapter wraps *slog.Logger to implement the Logger interface.
type slogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a Logger from *slog.Logger.
//
// Example:
//
//	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
//	adapter := tornago.NewSlogAdapter(logger)
//	cfg, _ := tornago.NewClientConfig(
//	    tornago.WithClientSocksAddr("127.0.0.1:9050"),
//	    tornago.WithLogger(adapter),
//	)
func NewSlogAdapter(logger *slog.Logger) Logger {
	if logger == nil {
		return noopLogger{}
	}
	return &slogAdapter{logger: logger}
}

func (s *slogAdapter) Log(level string, msg string, keysAndValues ...any) {
	switch level {
	case "debug":
		s.logger.Debug(msg, keysAndValues...)
	case "info":
		s.logger.Info(msg, keysAndValues...)
	case "warn":
		s.logger.Warn(msg, keysAndValues...)
	case "error":
		s.logger.Error(msg, keysAndValues...)
	default:
		s.logger.Info(msg, keysAndValues...)
	}
}
