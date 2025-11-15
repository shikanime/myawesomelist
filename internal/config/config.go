package config

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"log/slog"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct{ v *viper.Viper }

func New() *Config {
	vv := viper.New()
	vv.AutomaticEnv()
	return &Config{v: vv}
}

// GetDsn resolves the final DSN using env vars
func (c *Config) GetDsn() (*url.URL, error) {
	source := c.v.GetString("DSN")
	if source == "" {
		user := c.v.GetString("PGUSER")
		if user == "" {
			user = c.v.GetString("USER")
		}
		if user == "" {
			user = "postgres"
		}

		dbName := c.v.GetString("PGDATABASE")
		if dbName == "" {
			dbName = "postgres"
		}

		host := c.v.GetString("PGHOST")
		if host == "" {
			host = "localhost"
		}

		port := c.v.GetString("PGPORT")
		hasPortEnv := port != ""
		if !hasPortEnv || port == "" {
			port = "5432"
		}

		if strings.HasPrefix(host, "/") {
			socketDir := host

			// If PGHOST points to a file, derive directory and only infer port when PGPORT isn't set.
			if fi, err := os.Stat(host); err == nil && !fi.IsDir() {
				socketDir = filepath.Dir(host)
				if !hasPortEnv {
					base := filepath.Base(host)
					// Expected filename pattern: ".s.PGSQL.<port>"
					if strings.HasPrefix(base, ".s.PGSQL.") {
						if inferred := strings.TrimPrefix(base, ".s.PGSQL."); inferred != "" {
							if _, err := strconv.Atoi(inferred); err == nil {
								port = inferred
							}
						}
					}
				}
			}

			q := url.Values{}
			q.Set("host", socketDir)
			q.Set("port", port)
			q.Set("sslmode", "disable")
			source = "postgres://" + user + "@/" + dbName + "?" + q.Encode()
		} else {
			source = "postgres://" + user + "@" + host + ":" + port + "/" + dbName + "?sslmode=disable"
		}
	}

	u, err := url.Parse(source)
	if err != nil || u.Scheme == "" {
		return nil, errors.New("invalid DSN: must be in format driver://dataSourceName")
	}
	return u, nil
}

func (c *Config) GetGitHubToken() string {
	if t := c.v.GetString("GITHUB_TOKEN"); t != "" {
		return t
	}
	return c.v.GetString("GH_TOKEN")
}

func (c *Config) GetAddr() string {
	port := c.v.GetString("PORT")
	if port == "" {
		port = "8080"
	}
	host := c.v.GetString("HOST")
	if host == "" {
		host = "localhost"
	}
	return host + ":" + port
}

// GetCollectionCacheTTL returns the TTL for collection cache entries.
// Reads duration from env var COLLECTION_CACHE_TTL; defaults to 24h.
func (c *Config) GetCollectionCacheTTL() time.Duration {
	const def = 24 * time.Hour
	if v := c.v.GetString("COLLECTION_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// GetProjectStatsTTL returns the TTL for project stats cache entries.
// Reads duration from env var PROJECT_STATS_TTL; defaults to 6h.
func (c *Config) GetProjectStatsTTL() time.Duration {
	const def = 6 * time.Hour
	if v := c.v.GetString("PROJECT_STATS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// GetOpenAIAPIBaseURL returns the OpenAI API base URL from env var OPENAI_API_BASE_URL.
// Defaults to "https://api.openai.com/v1".
func (c *Config) GetOpenAIBaseURL() string { return c.v.GetString("OPENAI_BASE_URL") }

// GetOpenAIAPIKey returns the OpenAI API key from env var OPENAI_API_KEY.
func (c *Config) GetOpenAIAPIKey() string { return c.v.GetString("OPENAI_API_KEY") }

// GetEmbeddingModel returns the OpenAI embedding model from env var EMBEDDING_MODEL.
func (c *Config) GetEmbeddingModel() string { return c.v.GetString("EMBEDDING_MODEL") }
func (c *Config) Set(key string, value any) { c.v.Set(key, value) }

// GetLogLevel returns the log level from env var LOG_LEVEL mapped to slog.Level.
// Recognized values: debug, info (default), warn|warning, error.
func (c *Config) GetLogLevel() slog.Level {
	switch strings.ToLower(c.v.GetString("LOG_LEVEL")) {
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

// OnLogLevelChange calls fn with the slog.Level whenever it changes.
// The initial call is made immediately.
func (c *Config) OnLogLevelChange(fn func(slog.Level)) {
	apply := func() { fn(c.GetLogLevel()) }
	apply()
	c.v.OnConfigChange(func(e fsnotify.Event) { apply() })
}

// BindFlags binds all flags in the given FlagSet to this config's Viper instance.
// Flag binding helpers removed

// Watch watches for changes in the config file and env vars.
func (c *Config) Watch(ctx context.Context) {
	c.v.WatchConfig()
	go func() { <-ctx.Done() }()
}
