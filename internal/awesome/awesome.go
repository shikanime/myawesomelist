package awesome

import (
	"context"
	"fmt"

	"myawesomelist.shikanime.studio/internal/awesome/github"
	"myawesomelist.shikanime.studio/internal/config"
	"myawesomelist.shikanime.studio/internal/database"
)

// Awesome aggregates external clients used by the application.
type Awesome struct {
	db   *database.Database
	cfg  *config.Config
	opts ClientSetOptions
}

// ClientSetOptions holds configuration for initializing Awesome.
type ClientSetOptions struct {
	github []github.GitHubClientOption
}

// ClientSetOption applies a configuration to ClientSetOptions.
type ClientSetOption func(*ClientSetOptions)

// WithGitHubOptions forwards GitHub client options into the Awesome configuration.
func WithGitHubOptions(opts ...github.GitHubClientOption) ClientSetOption {
	return func(o *ClientSetOptions) { o.github = append(o.github, opts...) }
}

func NewForConfig(cfg *config.Config) (*Awesome, error) {
	var opts []ClientSetOption
	if token := cfg.GetGitHubToken(); token != "" {
		opts = append(
			opts,
			WithGitHubOptions(
				github.WithToken(token),
				github.WithLimiter(github.NewGitHubLimiter(true)),
				github.WithCollectionCacheTTL(cfg.GetCollectionCacheTTL()),
				github.WithProjectStatsTTL(cfg.GetProjectStatsTTL()),
			),
		)
	}
	db, err := database.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return New(db, cfg, opts...), nil
}

// New constructs an Awesome with the given database and options.
func New(db *database.Database, cfg *config.Config, opts ...ClientSetOption) *Awesome {
	var o ClientSetOptions
	for _, opt := range opts {
		opt(&o)
	}
	return &Awesome{db: db, cfg: cfg, opts: o}
}

// GitHub returns the configured GitHub client, or nil if not set.
func (aw *Awesome) GitHub() *github.Client {
	return github.NewClient(aw.db, aw.opts.github...)
}

// Core returns the core datastore-backed API, or nil if not set.
func (aw *Awesome) Core() *Core {
	return NewCoreClient(aw.db)
}

func (aw *Awesome) Close() error {
	if aw.db != nil {
		return aw.db.Close()
	}
	return nil
}

// Ping verifies that all configured clients are reachable.
func (aw *Awesome) Ping(ctx context.Context) error {
	if aw.db == nil {
		return fmt.Errorf("datastore not configured")
	}
	if err := aw.db.Ping(ctx); err != nil {
		return fmt.Errorf("datastore ping failed: %w", err)
	}
	return nil
}
