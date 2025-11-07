package awesome

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/go-github/v75/github"
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
	return base64.StdEncoding.DecodeString(*file.Content)
}

// GetCollection fetches a project collection from a single awesome repository
func (c *GitHubClient) GetCollection(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
	opts ...Option,
) (*myawesomelistv1.Collection, error) {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	// First, try to get the collection from the datastore
	col, err := c.d.GetCollection(ctx, repo)
	if err != nil {
		slog.Warn("Failed to query datastore for collection",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"error", err)
	}
	if col != nil {
		slog.Info("Retrieved collection from datastore cache",
			"hostname", repo.Hostname,
			"owner", repo.Owner,
			"repo", repo.Repo,
			"categories", len(col.Categories))
		return col, nil
	}

	// Collection not found in datastore or is stale, fetch from GitHub API
	slog.Info("Fetching collection from GitHub API",
		"hostname", repo.Hostname,
		"owner", repo.Owner,
		"repo", repo.Repo)

	content, err := c.GetReadme(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to read content for %s/%s: %v", repo.Owner, repo.Repo, err)
	}

	// Parse using encoding package with embedded options
	encCol, err := encoding.UnmarshallCollection(content, options.eopts...)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse collection for %s/%s: %v",
			repo.Owner,
			repo.Repo,
			err,
		)
	}

	col = &myawesomelistv1.Collection{
		Language:   encCol.Language,
		Categories: make([]*myawesomelistv1.Category, len(encCol.Categories)),
	}
	for i, category := range encCol.Categories {
		col.Categories[i] = &myawesomelistv1.Category{
			Name:     category.Name,
			Projects: make([]*myawesomelistv1.Project, 0),
		}
		for _, project := range category.Projects {
			col.Categories[i].Projects = append(
				col.Categories[i].Projects,
				&myawesomelistv1.Project{
					Name:        project.Name,
					Description: project.Description,
					Repo:        project.Repo,
				},
			)
		}
	}

	if err := c.d.UpSertCollection(ctx, repo, col); err != nil {
		slog.Warn("Failed to upsert collection",
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
	stats, err := c.d.GetProjectStats(ctx, repo)
	if err != nil {
		slog.Warn("Failed to query project stats from datastore",
			"owner", repo.Owner,
			"repo", repo.Repo,
			"error", err)
	}
	if stats != nil {
		return stats, nil
	}

	// Fetch from GitHub
	if err = c.l.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}
	ghRepo, _, err := c.c.Repositories.Get(ctx, repo.Owner, repo.Repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo info for %s/%s: %w", repo.Owner, repo.Repo, err)
	}

	stats = &myawesomelistv1.ProjectStats{
		StargazersCount: ptr.To(int32(ptr.Deref(ghRepo.StargazersCount, 0))),
		OpenIssueCount:  ptr.To(int32(ptr.Deref(ghRepo.OpenIssuesCount, 0))),
	}

	// Persist stats
	if err := c.d.UpSertProjectStats(ctx, repo, stats); err != nil {
		slog.Warn("Failed to upsert project stats",
			"owner", repo.Owner,
			"repo", repo.Repo,
			"error", err)
	}

	return stats, nil
}
