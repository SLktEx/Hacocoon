package main

import (
	"log/slog"
	"os"

	"github.com/SLktEx/Hacocoon/internal/logging"
)

func init() {
	logger, err := logging.NewFromEnv(os.Stderr)
	if err != nil {
		logger, _ = logging.New(logging.Config{Writer: os.Stderr, Level: slog.LevelInfo, Format: logging.FormatText})
		logger.Warn("invalid logging configuration; using defaults", "component", "control", "error", err)
	}
	logging.SetRoot(logger)
}
