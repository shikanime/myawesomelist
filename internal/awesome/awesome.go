package awesome

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"myawesomelist.shikanime.studio/internal/agent"
	"myawesomelist.shikanime.studio/internal/agent/openai"
	"myawesomelist.shikanime.studio/internal/awesome/core"
	"myawesomelist.shikanime.studio/internal/awesome/github"
	"myawesomelist.shikanime.studio/internal/config"
	"myawesomelist.shikanime.studio/internal/database"
)

// Awesome aggregates external clients used by the application.
type Awesome struct {
	db   *database.Database
	opts ClientSetOptions
}

// ClientSetOptions holds configuration for initializing Awesome.
type ClientSetOptions struct {
	github     []github.GitHubClientOption
	embeddings []agent.EmbeddingsOption
}

// ClientSetOption applies a configuration to ClientSetOptions.
type ClientSetOption func(*ClientSetOptions)

// WithGitHubOptions forwards GitHub client options into the Awesome configuration.
func WithGitHubOptions(opts ...github.GitHubClientOption) ClientSetOption {
	return func(o *ClientSetOptions) { o.github = append(o.github, opts...) }
}

// WithEmbeddingsOptions forwards OpenAI embeddings options into the Awesome configuration.
func WithEmbeddingsOptions(opts ...agent.EmbeddingsOption) ClientSetOption {
	return func(o *ClientSetOptions) { o.embeddings = append(o.embeddings, opts...) }
}

// NewForConfig initializes Awesome with the given config.
func NewForConfig(cfg *config.Config) (*Awesome, error) {
	if err := cfg.Bind(); err != nil {
		return nil, err
	}
	var opts []ClientSetOption
	if token := cfg.GetOpenAIAPIKey(); token != "" {
		opts = append(
			opts,
			WithEmbeddingsOptions(
				agent.WithLimiter(openai.NewOpenAIScalewayLimiter(cfg.GetScalewayVerified())),
			),
		)
	}
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
	return New(db, opts...), nil
}

// NewForConfigWithOptions builds Awesome with cfg and forwards embeddings options to the database.
func NewForConfigWithOptions(cfg *config.Config, opts ...ClientSetOption) (*Awesome, error) {
	var o ClientSetOptions
	for _, opt := range opts {
		opt(&o)
	}
	db, err := database.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return New(db, opts...), nil
}

// New constructs an Awesome with the given database and options.
func New(db *database.Database, opts ...ClientSetOption) *Awesome {
	var o ClientSetOptions
	for _, opt := range opts {
		opt(&o)
	}
	return &Awesome{db: db, opts: o}
}

// GitHub returns the configured GitHub client, or nil if not set.
func (aw *Awesome) GitHub() *github.Client {
	return github.NewClient(aw.db, aw.opts.github...)
}

func (aw *Awesome) Agent() *core.Agent {
	emb := agent.NewEmbeddingsForConfig(config.New(), aw.opts.embeddings...)
	return core.NewAgentClient(aw.db, emb)
}

func (aw *Awesome) Close() error {
	if aw.db != nil {
		return aw.db.Close()
	}
	return nil
}

// Ping verifies that all configured clients are reachable.
func (aw *Awesome) Ping(ctx context.Context) error {
	tracer := otel.Tracer("myawesomelist/core")
	ctx, span := tracer.Start(ctx, "Awesome.Ping")
	defer span.End()
	if aw.db == nil {
		return fmt.Errorf("datastore not configured")
	}
	if err := aw.db.Ping(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("datastore ping failed: %w", err)
	}
	return nil
}
