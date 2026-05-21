// Package logging sets up structured JSON logging to stdout.
package logging

import (
	"log/slog"
	"os"
)

// Setup returns a JSON slog.Logger and installs it as the default.
func Setup(level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	l := slog.New(h)
	slog.SetDefault(l)
	return l
}
