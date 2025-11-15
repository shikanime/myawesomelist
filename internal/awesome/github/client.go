package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/go-github/v75/github"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
	"k8s.io/utils/ptr"
	"myawesomelist.shikanime.studio/internal/database"
	"myawesomelist.shikanime.studio/internal/encoding"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// NewGitHubLimiter returns a rate limiter tuned for authenticated or unauthenticated GitHub API usage.
func NewGitHubLimiter(authenticated bool) *rate.Limiter {
	var limiter *rate.Limiter
	if authenticated {
		limiter = rate.NewLimiter(rate.Every(time.Hour), 5000)
		slog.Info(
			"Created authenticated GitHub rate limiter",
			"rate",
			"5000 requests/hour",
			"burst",
			10,
		)
	} else {
		limiter = rate.NewLimiter(rate.Every(time.Hour), 60)
		slog.Info("Created unauthenticated GitHub rate limiter", "rate", "60 requests/hour", "burst", 1)
	}
	return limiter
}

// Client wraps the GitHub API client with rate limiting and datastore access.
type Client struct {
	c    *github.Client
	l    *rate.Limiter
	d    *database.Database
	cttl time.Duration
	pttl time.Duration
}

// GitHubClientOptions configures the GitHub client.
type GitHubClientOptions struct {
	token   string
	limiter *rate.Limiter
	cttl    time.Duration
	pttl    time.Duration
}

// GitHubClientOption applies a configuration to GitHubClientOptions.
type GitHubClientOption func(*GitHubClientOptions)

// WithToken sets the personal access token for authenticated requests.
func WithToken(token string) GitHubClientOption {
	return func(o *GitHubClientOptions) { o.token = token }
}

// WithLimiter sets the rate limiter used for API calls.
func WithLimiter(l *rate.Limiter) GitHubClientOption {
	return func(o *GitHubClientOptions) { o.limiter = l }
}

// WithCollectionCacheTTL sets the collection cache TTL; zero means infinite (no refresh).
func WithCollectionCacheTTL(d time.Duration) GitHubClientOption {
	return func(o *GitHubClientOptions) { o.cttl = d }
}

// WithProjectStatsTTL sets the project stats cache TTL; zero means infinite (no refresh).
func WithProjectStatsTTL(d time.Duration) GitHubClientOption {
	return func(o *GitHubClientOptions) { o.pttl = d }
}

// NewClient constructs a GitHub Client with the given datastore and options.
func NewClient(db *database.Database, opts ...GitHubClientOption) *Client {
	var o GitHubClientOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.token != "" {
		slog.Info("Using authenticated GitHub client")
		return &Client{
			c:    github.NewClient(nil).WithAuthToken(o.token),
			l:    o.limiter,
			d:    db,
			cttl: o.cttl,
			pttl: o.pttl,
		}
	}
	slog.Warn("Using unauthenticated GitHub client (rate limited)")
	return &Client{c: github.NewClient(nil), l: o.limiter, d: db, cttl: o.cttl, pttl: o.pttl}
}

// GetReadme retrieves and decodes the README.md file for the given repository.
func (c *Client) GetReadme(ctx context.Context, repo *myawesomelistv1.Repository) ([]byte, error) {
	if err := c.l.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}
	file, _, _, err := c.c.Repositories.GetContents(ctx, repo.Owner, repo.Repo, "README.md", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %v", err)
	}
	return base64.StdEncoding.DecodeString(*file.Content)
}

type GetCollectionOption func(*getCollectionOptions)

type getCollectionOptions struct{ eopts []encoding.Option }

func WithStartSection(section string) GetCollectionOption {
	return func(o *getCollectionOptions) { o.eopts = append(o.eopts, encoding.WithStartSection(section)) }
}
func WithEndSection(section string) GetCollectionOption {
	return func(o *getCollectionOptions) { o.eopts = append(o.eopts, encoding.WithEndSection(section)) }
}
func WithSubsectionAsCategory() GetCollectionOption {
	return func(o *getCollectionOptions) { o.eopts = append(o.eopts, encoding.WithSubsectionAsCategory()) }
}

// ListCollections returns collections for the requested repositories, fetching from GitHub if not cached.
func (c *Client) ListCollections(
	ctx context.Context,
	repos []*myawesomelistv1.Repository,
	opts ...GetCollectionOption,
) ([]*myawesomelistv1.Collection, error) {
	cols, err := c.d.ListCollections(ctx, repos)
	if err != nil {
		slog.WarnContext(ctx, "Failed to list collections from datastore", "error", err)
	}
	colsByKey := make(map[string]*myawesomelistv1.Collection, len(cols))
	for _, col := range cols {
		if col != nil {
			key, err := url.JoinPath(col.Repo.Hostname, col.Repo.Owner, col.Repo.Repo)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to join path for %s/%s: %w",
					col.Repo.Owner,
					col.Repo.Repo,
					err,
				)
			}
			colsByKey[key] = col
		}
	}
	wg := errgroup.Group{}
	for _, r := range repos {
		wg.Go(func() error {
			key, err := url.JoinPath(r.Hostname, r.Owner, r.Repo)
			if err != nil {
				return fmt.Errorf("failed to join path for %s/%s: %w", r.Owner, r.Repo, err)
			}
			if _, ok := colsByKey[key]; ok {
				return nil
			}
			col, getErr := c.GetCollection(ctx, r, opts...)
			if getErr != nil {
				slog.WarnContext(
					ctx,
					"Failed to get collection",
					"hostname",
					r.Hostname,
					"owner",
					r.Owner,
					"repo",
					r.Repo,
					"error",
					getErr,
				)
				return nil
			}
			cols = append(cols, col)
			return nil
		})
	}
	if err := wg.Wait(); err != nil {
		return nil, err
	}
	return cols, nil
}

// GetCollection returns a single collection, honoring cache TTL semantics (zero TTL disables refresh).
func (c *Client) GetCollection(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
	opts ...GetCollectionOption,
) (*myawesomelistv1.Collection, error) {
	options := &getCollectionOptions{}
	for _, opt := range opts {
		opt(options)
	}
	col, err := c.d.GetCollection(ctx, repo)
	if err != nil {
		slog.WarnContext(
			ctx,
			"Failed to query datastore for collection",
			"hostname",
			repo.Hostname,
			"owner",
			repo.Owner,
			"repo",
			repo.Repo,
			"error",
			err,
		)
	}
	if col != nil {
		ttl := c.cttl
		if ttl > 0 {
			if time.Since(col.UpdatedAt.AsTime()) < ttl {
				slog.InfoContext(
					ctx,
					"Collection cache fresh; skip GitHub fetch",
					"hostname", repo.Hostname,
					"owner", repo.Owner,
					"repo", repo.Repo,
					"categories", len(col.Categories),
					"updated_at", col.UpdatedAt.AsTime(),
					"ttl", ttl,
				)
				return col, nil
			}
			slog.InfoContext(
				ctx,
				"Collection cache stale; refetching from GitHub",
				"hostname", repo.Hostname,
				"owner", repo.Owner,
				"repo", repo.Repo,
				"updated_at", col.UpdatedAt.AsTime(),
				"ttl", ttl,
			)
		} else {
			return col, nil
		}
	}
	if col != nil {
		slog.InfoContext(
			ctx,
			"Retrieved collection from datastore cache",
			"hostname",
			repo.Hostname,
			"owner",
			repo.Owner,
			"repo",
			repo.Repo,
			"categories",
			len(col.Categories),
			"updated_at",
			col.UpdatedAt.AsTime(),
		)
		return col, nil
	}
	slog.InfoContext(
		ctx,
		"Fetching collection from GitHub API",
		"hostname",
		repo.Hostname,
		"owner",
		repo.Owner,
		"repo",
		repo.Repo,
	)
	content, err := c.GetReadme(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to read content for %s/%s: %v", repo.Owner, repo.Repo, err)
	}
	rms, idErr := c.d.UpsertRepositories(ctx, []*myawesomelistv1.Repository{repo})
	if idErr != nil {
		slog.WarnContext(
			ctx,
			"Failed to resolve repository id",
			"hostname",
			repo.Hostname,
			"owner",
			repo.Owner,
			"repo",
			repo.Repo,
			"error",
			idErr,
		)
	} else if err = c.d.UpsertProjectMetadata(ctx, []*database.ProjectMetadata{{RepositoryID: rms[0].ID, Readme: string(content)}}); err != nil {
		slog.WarnContext(ctx, "Failed to upsert project metadata", "hostname", repo.Hostname, "owner", repo.Owner, "repo", repo.Repo, "error", err)
	}
	encCol, err := encoding.UnmarshallCollection(content, options.eopts...)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse collection for %s/%s: %v",
			repo.Owner,
			repo.Repo,
			err,
		)
	}
	col = encCol.ToProto(repo)
	if err := c.d.UpsertCollections(ctx, []*myawesomelistv1.Collection{col}); err != nil {
		slog.WarnContext(
			ctx,
			"Failed to upsert collection",
			"hostname",
			repo.Hostname,
			"owner",
			repo.Owner,
			"repo",
			repo.Repo,
			"error",
			err,
		)
	}
	return col, nil
}

// GetProjectStats returns repository statistics, honoring cache TTL semantics (zero TTL disables refresh).
func (c *Client) GetProjectStats(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*myawesomelistv1.ProjectStats, error) {
	stats, err := c.d.GetProjectStats(ctx, repo)
	if err != nil {
		slog.WarnContext(
			ctx,
			"Failed to query project stats from datastore",
			"hostname",
			repo.Hostname,
			"owner",
			repo.Owner,
			"repo",
			repo.Repo,
			"error",
			err,
		)
	}
	if stats != nil {
		ttl := c.pttl
		if ttl > 0 {
			if time.Since(stats.UpdatedAt.AsTime()) < ttl {
				slog.InfoContext(
					ctx,
					"Project stats cache fresh; skip GitHub fetch",
					"hostname", repo.Hostname,
					"owner", repo.Owner,
					"repo", repo.Repo,
					"updated_at", stats.UpdatedAt.AsTime(),
					"ttl", ttl,
				)
				return stats, nil
			}
			slog.InfoContext(
				ctx,
				"Project stats cache stale; refetching from GitHub",
				"hostname", repo.Hostname,
				"owner", repo.Owner,
				"repo", repo.Repo,
				"updated_at", stats.UpdatedAt.AsTime(),
				"ttl", ttl,
			)
		} else {
			return stats, nil
		}
	}
	if err = c.l.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}
	ghRepo, _, err := c.c.Repositories.Get(ctx, repo.Owner, repo.Repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo info for %s/%s: %w", repo.Owner, repo.Repo, err)
	}
	stats = &myawesomelistv1.ProjectStats{
		StargazersCount: ptr.To(uint32(ptr.Deref(ghRepo.StargazersCount, 0))),
		OpenIssueCount:  ptr.To(uint32(ptr.Deref(ghRepo.OpenIssuesCount, 0))),
	}
	rms, idErr := c.d.UpsertRepositories(ctx, []*myawesomelistv1.Repository{repo})
	if idErr != nil {
		slog.WarnContext(
			ctx,
			"Failed to resolve repository id",
			"hostname",
			repo.Hostname,
			"owner",
			repo.Owner,
			"repo",
			repo.Repo,
			"error",
			idErr,
		)
	} else if err := c.d.UpsertProjectStats(ctx, []*database.ProjectStats{{RepositoryID: rms[0].ID, StargazersCount: stats.StargazersCount, OpenIssueCount: stats.OpenIssueCount}}); err != nil {
		slog.WarnContext(ctx, "Failed to upsert project stats", "hostname", repo.Hostname, "owner", repo.Owner, "repo", repo.Repo, "error", err)
	}
	return stats, nil
}
