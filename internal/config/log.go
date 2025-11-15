package config

import (
	"log/slog"
	"os"
)

func SetupLog(cfg *Config) {
	var lv slog.LevelVar
	cfg.OnLogLevelChange(func(level slog.Level) { lv.Set(level) })
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &lv})))
}
