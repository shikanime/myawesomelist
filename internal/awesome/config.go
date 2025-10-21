package awesome

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GitHubRepoConfig represents configuration for a GitHub repository
type GitHubRepoConfig struct {
	Owner   string
	Repo    string
	Options []Option
}

// DefaultGitHubRepos contains the default list of awesome repositories to fetch
var DefaultGitHubRepos = []GitHubRepoConfig{
	{
		Owner: "avelino",
		Repo:  "awesome-go",
		Options: []Option{
			WithStartSection("Actor Model"),
		},
	},
	{
		Owner: "h4cc",
		Repo:  "awesome-elixir",
		Options: []Option{
			WithStartSection("Actors"),
		},
	},
	{
		Owner: "sorrycc",
		Repo:  "awesome-javascript",
		Options: []Option{
			WithStartSection("Package Managers"),
			WithEndSection("Worth Reading"),
		},
	},
	{
		Owner: "gostor",
		Repo:  "awesome-go-storage",
		Options: []Option{
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
