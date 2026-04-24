// Package logging centralizes slog setup. Call Setup once at program start;
// all subsequent calls to the stdlib top-level slog functions (slog.Info,
// slog.Error, etc.) use the configured handler.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup configures slog's default logger. levelName is one of
// "debug", "info", "warn", "error" (case-insensitive). Unknown values fall
// back to info.
func Setup(levelName string) {
	level := parseLevel(levelName)
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
