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
	return &Config{v: vv}
}

func (c *Config) Bind() error {
	c.v.AutomaticEnv()
	if err := c.v.BindEnv("dsn", "DSN"); err != nil {
		return err
	}
	if err := c.v.BindEnv("postgres_user", "PGUSER"); err != nil {
		return err
	}
	if err := c.v.BindEnv("user", "USER"); err != nil {
		return err
	}
	if err := c.v.BindEnv("postgres_database", "PGDATABASE"); err != nil {
		return err
	}
	if err := c.v.BindEnv("postgres_host", "PGHOST"); err != nil {
		return err
	}
	if err := c.v.BindEnv("postgres_port", "PGPORT"); err != nil {
		return err
	}
	if err := c.v.BindEnv("github_token", "GITHUB_TOKEN", "GH_TOKEN"); err != nil {
		return err
	}
	if err := c.v.BindEnv("port", "PORT"); err != nil {
		return err
	}
	if err := c.v.BindEnv("host", "HOST"); err != nil {
		return err
	}
	if err := c.v.BindEnv("collection_cache_ttl", "COLLECTION_CACHE_TTL"); err != nil {
		return err
	}
	if err := c.v.BindEnv("project_stats_ttl", "PROJECT_STATS_TTL"); err != nil {
		return err
	}
	if err := c.v.BindEnv("project_embeddings_ttl", "PROJECT_EMBEDDINGS_TTL"); err != nil {
		return err
	}
	if err := c.v.BindEnv("openai_base_url", "OPENAI_BASE_URL"); err != nil {
		return err
	}
	if err := c.v.BindEnv("openai_api_key", "OPENAI_API_KEY"); err != nil {
		return err
	}
	if err := c.v.BindEnv("embedding_model", "EMBEDDING_MODEL"); err != nil {
		return err
	}
	if err := c.v.BindEnv("log_level", "LOG_LEVEL"); err != nil {
		return err
	}
	if err := c.v.BindEnv("scaleway_verified", "SCALEWAY_VERIFIED"); err != nil {
		return err
	}
	if err := c.v.BindEnv("otel_service_name", "OTEL_SERVICE_NAME"); err != nil {
		return err
	}
	if err := c.v.BindEnv("service_name", "SERVICE_NAME"); err != nil {
		return err
	}
	return nil
}

// GetDsn resolves the final DSN using env vars
func (c *Config) GetDsn() (*url.URL, error) {
	source := c.v.GetString("dsn")
	if source == "" {
		user := c.v.GetString("postgres_user")
		if user == "" {
			user = c.v.GetString("user")
		}
		if user == "" {
			user = "postgres"
		}

		dbName := c.v.GetString("postgres_database")
		if dbName == "" {
			dbName = "postgres"
		}

		host := c.v.GetString("postgres_host")
		if host == "" {
			host = "localhost"
		}

		port := c.v.GetString("postgres_port")
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
	return c.v.GetString("github_token")
}

func (c *Config) GetAddr() string {
	port := c.v.GetString("port")
	if port == "" {
		port = "8080"
	}
	host := c.v.GetString("host")
	if host == "" {
		host = "localhost"
	}
	return host + ":" + port
}

// GetCollectionCacheTTL returns the TTL for collection cache entries.
// Reads duration from env var COLLECTION_CACHE_TTL; defaults to 24h.
func (c *Config) GetCollectionCacheTTL() time.Duration {
	const def = 24 * time.Hour
	if v := c.v.GetString("collection_cache_ttl"); v != "" {
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
	if v := c.v.GetString("project_stats_ttl"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func (c *Config) GetProjectEmbeddingsTTL() time.Duration {
	if v := c.v.GetString("project_embeddings_ttl"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return -1
}

// GetOpenAIAPIBaseURL returns the OpenAI API base URL from env var OPENAI_API_BASE_URL.
// Defaults to "https://api.openai.com/v1".
func (c *Config) GetOpenAIBaseURL() string { return c.v.GetString("openai_base_url") }

// GetOpenAIAPIKey returns the OpenAI API key from env var OPENAI_API_KEY.
func (c *Config) GetOpenAIAPIKey() string { return c.v.GetString("openai_api_key") }

// GetEmbeddingModel returns the OpenAI embedding model from env var EMBEDDING_MODEL.
func (c *Config) GetEmbeddingModel() string { return c.v.GetString("embedding_model") }
func (c *Config) Set(key string, value any) { c.v.Set(key, value) }

// GetLogLevel returns the log level from env var LOG_LEVEL mapped to slog.Level.
// Recognized values: debug, info (default), warn|warning, error.
func (c *Config) GetLogLevel() slog.Level {
	switch strings.ToLower(c.v.GetString("log_level")) {
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
	c.v.OnConfigChange(func(e fsnotify.Event) { fn(c.GetLogLevel()) })
}

// GetScalewayVerified returns the Scaleway verified flag from env var SCALEWAY_VERIFIED.
func (c *Config) GetScalewayVerified() bool { return c.v.GetBool("scaleway_verified") }

// Watch watches for changes in the config file and env vars.
func (c *Config) Watch(ctx context.Context) {
	c.v.WatchConfig()
	go func() { <-ctx.Done() }()
}

func (c *Config) GetServiceName() string {
	if v := c.v.GetString("otel_service_name"); v != "" {
		return v
	}
	if v := c.v.GetString("service_name"); v != "" {
		return v
	}
	return "myawesomelist"
}
