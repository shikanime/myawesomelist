package awesome

import (
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// GitHubRepoConfig represents configuration for a GitHub repository
type GitHubRepoConfig struct {
	Repo    *myawesomelistv1.Repository
	Options []GetCollectionOption
}

// DefaultGitHubRepos contains the default list of awesome repositories to fetch
var DefaultGitHubRepos = []GitHubRepoConfig{
	{
		Repo: &myawesomelistv1.Repository{
			Hostname: "github.com",
			Owner:    "avelino",
			Repo:     "awesome-go",
		},
		Options: []GetCollectionOption{
			WithStartSection("Actor Model"),
			WithSubsectionAsCategory(),
		},
	},
	{
		Repo: &myawesomelistv1.Repository{
			Hostname: "github.com",
			Owner:    "h4cc",
			Repo:     "awesome-elixir",
		},
		Options: []GetCollectionOption{
			WithStartSection("Actors"),
		},
	},
	{
		Repo: &myawesomelistv1.Repository{
			Hostname: "github.com",
			Owner:    "sorrycc",
			Repo:     "awesome-javascript",
		},
		Options: []GetCollectionOption{
			WithStartSection("Package Managers"),
			WithEndSection("Worth Reading"),
		},
	},
	{
		Repo: &myawesomelistv1.Repository{
			Hostname: "github.com",
			Owner:    "gostor",
			Repo:     "awesome-go-storage",
		},
		Options: []GetCollectionOption{
			WithStartSection("Storage Server"),
		},
	},
}

// GetDsn resolves the final DSN using env vars
func GetDsn() (*url.URL, error) {
	source := os.Getenv("DSN")
	if source == "" {
		user := os.Getenv("PGUSER")
		if user == "" {
			user = os.Getenv("USER")
		}
		if user == "" {
			user = "postgres"
		}

		dbName := os.Getenv("PGDATABASE")
		if dbName == "" {
			dbName = "postgres"
		}

		host := os.Getenv("PGHOST")
		if host == "" {
			host = "localhost"
		}

		port, hasPortEnv := os.LookupEnv("PGPORT")
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

func GetGitHubToken() string {
	// Prefer GITHUB_TOKEN; fall back to GH_TOKEN if present
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
}

func GetAddr() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	host := os.Getenv("HOST")
	if host == "" {
		host = "localhost"
	}
	return host + ":" + port
}

func GetLogLevel() *slog.LevelVar {
	lv := new(slog.LevelVar)
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		lv.Set(slog.LevelDebug)
	case "warn", "warning":
		lv.Set(slog.LevelWarn)
	case "error":
		lv.Set(slog.LevelError)
	case "info", "":
		lv.Set(slog.LevelInfo)
	default:
		lv.Set(slog.LevelInfo)
	}
	return lv
}

// GetCollectionCacheTTL returns the TTL for collection cache entries.
// Reads duration from env var COLLECTION_CACHE_TTL; defaults to 24h.
func GetCollectionCacheTTL() time.Duration {
	const def = 24 * time.Hour
	if v := os.Getenv("COLLECTION_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// GetProjectStatsTTL returns the TTL for project stats cache entries.
// Reads duration from env var PROJECT_STATS_TTL; defaults to 6h.
func GetProjectStatsTTL() time.Duration {
	const def = 6 * time.Hour
	if v := os.Getenv("PROJECT_STATS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// GetOpenAIAPIBaseURL returns the OpenAI API base URL from env var OPENAI_API_BASE_URL.
// Defaults to "https://api.openai.com/v1".
func GetOpenAIBaseURL() string {
	return os.Getenv("OPENAI_BASE_URL")
}

// GetOpenAIAPIKey returns the OpenAI API key from env var OPENAI_API_KEY.
func GetOpenAIAPIKey() string {
	return os.Getenv("OPENAI_API_KEY")
}

// GetEmbeddingModel returns the OpenAI embedding model from env var EMBEDDING_MODEL.
func GetEmbeddingModel() string {
	return os.Getenv("EMBEDDING_MODEL")
}
