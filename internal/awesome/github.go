package awesome

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
	"myawesomelist.shikanime.studio/internal/encoding"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// NewGitHubLimiter creates a new rate limiter for GitHub API calls
// GitHub allows 5000 requests per hour for authenticated users (83.33 per minute)
// and 60 requests per hour for unauthenticated users (1 per minute)
func NewGitHubLimiter(authenticated bool) *rate.Limiter {
	var limiter *rate.Limiter

	if authenticated {
		limiter = rate.NewLimiter(rate.Every(time.Hour), 5000)
		slog.Info("Created authenticated GitHub rate limiter",
			"rate", "5000 requests/hour",
			"burst", 10)
	} else {
		limiter = rate.NewLimiter(rate.Every(time.Hour), 60)
		slog.Info("Created unauthenticated GitHub rate limiter",
			"rate", "60 requests/hour",
			"burst", 1)
	}

	return limiter
}

// GitHubClient represents a client for GitHub API operations
type GitHubClient struct {
	c *github.Client
	l *rate.Limiter
	d *DataStore
}

// GitHubClientOptions holds configuration for initializing a GitHubClient.
type GitHubClientOptions struct {
	token   string
	limiter *rate.Limiter
}

// GitHubClientOption applies a configuration to GitHubClientOptions.
type GitHubClientOption func(*GitHubClientOptions)

// WithToken sets the OAuth token used for authenticated GitHub requests.
func WithToken(token string) GitHubClientOption {
	return func(o *GitHubClientOptions) { o.token = token }
}

// WithLimiter sets a custom rate limiter for the GitHub client.
func WithLimiter(l *rate.Limiter) GitHubClientOption {
	return func(o *GitHubClientOptions) { o.limiter = l }
}

// NewGitHubClient creates a new GitHub client with optional authentication
func NewGitHubClient(ds *DataStore, opts ...GitHubClientOption) *GitHubClient {
	var o GitHubClientOptions
	for _, opt := range opts {
		opt(&o)
	}

	if o.token != "" {
		slog.Info("Using authenticated GitHub client")
		return &GitHubClient{
			c: github.NewClient(nil).WithAuthToken(o.token),
			l: o.limiter,
			d: ds,
		}
	}

	slog.Warn("Using unauthenticated GitHub client (rate limited)")
	return &GitHubClient{
		c: github.NewClient(nil),
		l: o.limiter,
		d: ds,
	}
}

// GetReadme creates a reader for the README.md file of the specified repository
func (c *GitHubClient) GetReadme(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) ([]byte, error) {
	slog.DebugContext(ctx, "GetReadme", "owner", repo.Owner, "repo", repo.Repo)
	if err := c.l.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}
	file, _, _, err := c.c.Repositories.GetContents(
		ctx,
		repo.Owner,
		repo.Repo,
		"README.md",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %v", err)
	}
	content, derr := base64.StdEncoding.DecodeString(*file.Content)
	if derr != nil {
		return nil, derr
	}
	slog.DebugContext(ctx, "GetReadme decoded", "bytes", len(content))
	return content, nil
}

// getCollectionOptions represents configuration getCollectionOptions for fetching data
type getCollectionOptions struct {
	eopts []encoding.Option
}

// GetCollectionOption is a function that configures Options
type GetCollectionOption func(*getCollectionOptions)

// WithStartSection overrides the start section for parsing categories
func WithStartSection(section string) GetCollectionOption {
	return func(o *getCollectionOptions) {
		o.eopts = append(o.eopts, encoding.WithStartSection(section))
	}
}

// WithEndSection overrides the end section for parsing categories
func WithEndSection(section string) GetCollectionOption {
	return func(o *getCollectionOptions) {
		o.eopts = append(o.eopts, encoding.WithEndSection(section))
	}
}

// WithSubsectionAsCategory enables H3 subsections to be treated as categories.
func WithSubsectionAsCategory() GetCollectionOption {
	return func(o *getCollectionOptions) {
		o.eopts = append(o.eopts, encoding.WithSubsectionAsCategory())
	}
}

// ListCollections loads collections for given repos: first from datastore,
// then fetches any missing via GetCollection (which also refreshes stale entries).
func (c *GitHubClient) ListCollections(
	ctx context.Context,
	repos []*myawesomelistv1.Repository,
	opts ...GetCollectionOption,
) ([]*myawesomelistv1.Collection, error) {
	cols, err := c.d.ListCollections(ctx, repos)
	if err != nil {
		slog.WarnContext(ctx, "Failed to list collections from datastore", "error", err)
	}
	slog.DebugContext(ctx, "ListCollections datastore results", "count", len(cols))

	// Index datastore results by repository key
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

	// Build output in the same order as input repos, fetching missing via
	// GetCollection
	wg := errgroup.Group{}
	for _, r := range repos {
		wg.Go(func() error {
			key, err := url.JoinPath(r.Hostname, r.Owner, r.Repo)
			if err != nil {
				return fmt.Errorf("failed to join path for %s/%s: %w", r.Owner, r.Repo, err)
			}

			if _, ok := colsByKey[key]; ok {
				slog.DebugContext(ctx, "ListCollections cache hit", "key", key)
				return nil
			}

			slog.DebugContext(ctx, "ListCollections cache miss; fetching", "key", key)
			col, getErr := c.GetCollection(ctx, r, opts...)
			if getErr != nil {
				slog.WarnContext(ctx, "Failed to get collection",
					"hostname", r.Hostname,
					"owner", r.Owner,
					"repo", r.Repo,
					"error", getErr)
				return nil
			}
			slog.DebugContext(
				ctx,
				"ListCollections fetched",
				"key",
				key,
				"categories",
				len(col.Categories),
			)
			cols = append(cols, col)
			return nil
		})
	}
	if err := wg.Wait(); err != nil {
		return nil, err
	}

	return cols, nil
}

// GetCollection fetches a project collection from a single awesome repository
func (c *GitHubClient) GetCollection(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
	opts ...GetCollectionOption,
) (*myawesomelistv1.Collection, error) {
	options := &getCollectionOptions{}
	for _, opt := range opts {
		opt(options)
	}
	slog.DebugContext(
		ctx,
		"GetCollection",
		"hostname",
		repo.Hostname,
		"owner",
		repo.Owner,
		"repo",
		repo.Repo,
	)

	// First, try to get the collection from the datastore with its last update time
	col, err := c.d.GetCollection(ctx, repo)
	if err != nil {
		slog.WarnContext(ctx, "Failed to query datastore for collection",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"error", err)
	}
	if col != nil {
		ttl := GetCollectionCacheTTL()
		slog.DebugContext(
			ctx,
			"GetCollection cache check",
			"updated_at",
			col.UpdatedAt.AsTime(),
			"ttl",
			ttl,
		)
		if time.Since(col.UpdatedAt.AsTime()) < ttl {
			slog.InfoContext(ctx, "Collection cache fresh; skip GitHub fetch",
				"hostname", repo.Hostname,
				"owner", repo.Owner,
				"repo", repo.Repo,
				"categories", len(col.Categories),
				"updated_at", col.UpdatedAt.AsTime(),
				"ttl", ttl)
			return col, nil
		}
		slog.InfoContext(ctx, "Collection cache stale; refetching from GitHub",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"updated_at", col.UpdatedAt.AsTime(),
			"ttl", ttl)
	}
	if col != nil {
		slog.InfoContext(ctx, "Retrieved collection from datastore cache",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"categories", len(col.Categories),
			"updated_at", col.UpdatedAt.AsTime())
		return col, nil
	}

	// Collection not found in datastore or is stale, fetch from GitHub API
	slog.InfoContext(ctx, "Fetching collection from GitHub API",
		"hostname", repo.Hostname,
		"owner", repo.Owner,
		"repo", repo.Repo)

	content, err := c.GetReadme(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to read content for %s/%s: %v", repo.Owner, repo.Repo, err)
	}
	slog.DebugContext(ctx, "GetCollection readme loaded", "bytes", len(content))

	rms, idErr := c.d.UpsertRepositories(ctx, []*myawesomelistv1.Repository{repo})
	if idErr != nil {
		slog.WarnContext(ctx, "Failed to resolve repository id",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"error", idErr)
	} else if err = c.d.UpsertProjectMetadata(ctx, []*ProjectMetadata{&ProjectMetadata{RepositoryID: rms[0].ID, Readme: string(content)}}); err != nil {
		slog.WarnContext(ctx, "Failed to upsert project metadata",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"error", err)
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
	slog.DebugContext(ctx, "GetCollection parsed", "categories", len(col.Categories))

	if err := c.d.UpsertCollections(ctx, []*myawesomelistv1.Collection{col}); err != nil {
		slog.WarnContext(ctx, "Failed to upsert collection",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"error", err)
	}

	return col, nil
}

// GetProjectStats retrieves cached stats or fetches from GitHub and persists them
func (c *GitHubClient) GetProjectStats(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*myawesomelistv1.ProjectStats, error) {
	slog.DebugContext(ctx, "GetProjectStats", "owner", repo.Owner, "repo", repo.Repo)
	stats, err := c.d.GetProjectStats(ctx, repo)
	if err != nil {
		slog.WarnContext(ctx, "Failed to query project stats from datastore",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"error", err)
	}
	if stats != nil {
		ttl := GetProjectStatsTTL()
		slog.DebugContext(
			ctx,
			"GetProjectStats cache check",
			"updated_at",
			stats.UpdatedAt.AsTime(),
			"ttl",
			ttl,
		)
		if time.Since(stats.UpdatedAt.AsTime()) < ttl {
			slog.InfoContext(ctx, "Project stats cache fresh; skip GitHub fetch",
				"hostname", repo.Hostname,
				"owner", repo.Owner,
				"repo", repo.Repo,
				"updated_at", stats.UpdatedAt.AsTime(),
				"ttl", ttl)
			return stats, nil
		}
		slog.InfoContext(ctx, "Project stats cache stale; refetching from GitHub",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"updated_at", stats.UpdatedAt.AsTime(),
			"ttl", ttl)
	}

	// Fetch from GitHub
	if err = c.l.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}
	slog.DebugContext(
		ctx,
		"GetProjectStats fetching GitHub API",
		"owner",
		repo.Owner,
		"repo",
		repo.Repo,
	)
	ghRepo, _, err := c.c.Repositories.Get(ctx, repo.Owner, repo.Repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo info for %s/%s: %w", repo.Owner, repo.Repo, err)
	}

	stats = &myawesomelistv1.ProjectStats{
		StargazersCount: ptr.To(uint32(ptr.Deref(ghRepo.StargazersCount, 0))),
		OpenIssueCount:  ptr.To(uint32(ptr.Deref(ghRepo.OpenIssuesCount, 0))),
	}
	slog.DebugContext(
		ctx,
		"GetProjectStats fetched",
		"stars",
		ptr.Deref(stats.StargazersCount, 0),
		"open_issues",
		ptr.Deref(stats.OpenIssueCount, 0),
	)

	// Persist stats
	rms, idErr := c.d.UpsertRepositories(ctx, []*myawesomelistv1.Repository{repo})
	if idErr != nil {
		slog.WarnContext(ctx, "Failed to resolve repository id",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"error", idErr)
	} else if err := c.d.UpsertProjectStats(ctx, []*ProjectStats{&ProjectStats{RepositoryID: rms[0].ID, StargazersCount: stats.StargazersCount, OpenIssueCount: stats.OpenIssueCount}}); err != nil {
		slog.WarnContext(ctx, "Failed to upsert project stats",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"error", err)
	}

	return stats, nil
}
