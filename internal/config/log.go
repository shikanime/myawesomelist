package config

import (
	"log/slog"
	"os"
)

// SetupLog configures a global slog logger whose level follows LOG_LEVEL changes.
func SetupLog(cfg *Config) {
	var lv slog.LevelVar
	lv.Set(cfg.GetLogLevel())
	cfg.OnLogLevelChange(func(level slog.Level) { lv.Set(level) })
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &lv})))
}
