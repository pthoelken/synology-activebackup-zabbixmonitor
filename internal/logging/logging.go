package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func New(levelName string, logDir string, toFile bool) (*slog.Logger, func(), error) {
	var level slog.Level
	switch strings.ToLower(levelName) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var writer io.Writer = os.Stdout
	closeFn := func() {}

	if toFile {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, closeFn, err
		}
		file, err := os.OpenFile(filepath.Join(logDir, "collector.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, closeFn, err
		}
		writer = file
		closeFn = func() {
			_ = file.Close()
		}
	}

	logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level}))
	return logger, closeFn, nil
}

func Redact(value string) string {
	if value == "" {
		return ""
	}
	if strings.Contains(value, "@") {
		parts := strings.SplitN(value, "@", 2)
		if len(parts[0]) <= 1 {
			return "***@" + parts[1]
		}
		return parts[0][:1] + "***@" + parts[1]
	}
	if len(value) <= 6 {
		return "***"
	}
	return value[:3] + "***" + value[len(value)-2:]
}
