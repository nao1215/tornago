package tornago

import (
	"bytes"
	"log/slog"
	"testing"
)

func TestNoopLogger(t *testing.T) {
	t.Parallel()

	logger := noopLogger{}

	// Should not panic
	logger.Log("debug", "test message")
	logger.Log("info", "test message", "key", "value")
	logger.Log("warn", "test message", "key1", "value1", "key2", "value2")
	logger.Log("error", "test message")
}

func TestNewSlogAdapterWithNil(t *testing.T) {
	t.Parallel()

	adapter := NewSlogAdapter(nil)

	// Should return noopLogger
	if _, ok := adapter.(noopLogger); !ok {
		t.Errorf("NewSlogAdapter(nil) should return noopLogger, got %T", adapter)
	}
}

func TestSlogAdapter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		level        string
		msg          string
		keysAndVals  []any
		wantContains string
	}{
		{
			name:         "should log debug message",
			level:        "debug",
			msg:          "debug message",
			keysAndVals:  []any{"key", "value"},
			wantContains: "debug message",
		},
		{
			name:         "should log info message",
			level:        "info",
			msg:          "info message",
			keysAndVals:  []any{"key", "value"},
			wantContains: "info message",
		},
		{
			name:         "should log warn message",
			level:        "warn",
			msg:          "warn message",
			keysAndVals:  []any{"key", "value"},
			wantContains: "warn message",
		},
		{
			name:         "should log error message",
			level:        "error",
			msg:          "error message",
			keysAndVals:  []any{"key", "value"},
			wantContains: "error message",
		},
		{
			name:         "should default to info for unknown level",
			level:        "unknown",
			msg:          "unknown level message",
			keysAndVals:  []any{},
			wantContains: "unknown level message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			slogLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))

			adapter := NewSlogAdapter(slogLogger)

			adapter.Log(tt.level, tt.msg, tt.keysAndVals...)

			output := buf.String()
			if output == "" && tt.level != "unknown" {
				t.Error("Log() produced no output")
			}

			// For unknown level, it should still log as INFO
			if tt.level == "unknown" && output == "" {
				t.Error("Log() with unknown level produced no output")
			}
		})
	}
}

func TestSlogAdapterWithMultipleKeyValues(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	slogLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	adapter := NewSlogAdapter(slogLogger)

	adapter.Log("info", "test message",
		"key1", "value1",
		"key2", 42,
		"key3", true,
	)

	output := buf.String()
	if output == "" {
		t.Error("Log() produced no output")
	}
}

func TestSlogAdapterImplementsLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	slogLogger := slog.New(slog.NewTextHandler(&buf, nil))

	adapter := NewSlogAdapter(slogLogger)

	// Should implement Logger interface - verify no panic
	adapter.Log("info", "test")
}

func TestNoopLoggerImplementsLogger(t *testing.T) {
	t.Parallel()

	var logger Logger = noopLogger{}

	// Should implement Logger interface - verify no panic
	logger.Log("debug", "test")
}
