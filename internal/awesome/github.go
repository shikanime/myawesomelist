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
	token string
}

// GitHubClientOption applies a configuration to GitHubClientOptions.
type GitHubClientOption func(*GitHubClientOptions)

// WithToken sets the OAuth token used for authenticated GitHub requests.
func WithToken(token string) GitHubClientOption {
	return func(o *GitHubClientOptions) { o.token = token }
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
			l: NewGitHubLimiter(true),
			d: ds,
		}
	}

	slog.Warn("Using unauthenticated GitHub client (rate limited)")
	return &GitHubClient{
		c: github.NewClient(nil),
		l: NewGitHubLimiter(false),
		d: ds,
	}
}

// GetReadme creates a reader for the README.md file of the specified repository
func (c *GitHubClient) GetReadme(ctx context.Context, owner string, repo string) ([]byte, error) {
	if err := c.l.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}
	file, _, _, err := c.c.Repositories.GetContents(
		ctx,
		owner,
		repo,
		"README.md",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %v", err)
	}
	return base64.StdEncoding.DecodeString(*file.Content)
}

// GetCollection fetches a project collection from a single awesome repository
func (c *GitHubClient) GetCollection(ctx context.Context, owner, repo string, opts ...Option) (*myawesomelistv1.Collection, error) {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	// First, try to get the collection from the datastore
	if cachedCollection, err := c.d.GetCollection(ctx, owner, repo); err != nil {
		slog.Warn("Failed to query datastore for collection",
			"owner", owner,
			"repo", repo,
			"error", err)
	} else if cachedCollection != nil {
		slog.Info("Retrieved collection from datastore cache",
			"owner", owner,
			"repo", repo,
			"categories", len(cachedCollection.Categories))
		return cachedCollection, nil
	}

	// Collection not found in datastore or is stale, fetch from GitHub API
	slog.Info("Fetching collection from GitHub API",
		"owner", owner,
		"repo", repo)

	content, err := c.GetReadme(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to read content for %s/%s: %v", owner, repo, err)
	}

	// Parse using encoding package with embedded options
	encColl, err := encoding.UnmarshallCollection(content, options.eopts...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse collection for %s/%s: %v", owner, repo, err)
	}

	categories := make([]*myawesomelistv1.Category, len(encColl.Categories))
	for i, encCat := range encColl.Categories {
		projects := make([]*myawesomelistv1.Project, len(encCat.Projects))
		for j, encProj := range encCat.Projects {
			projects[j] = &myawesomelistv1.Project{
				Name:        encProj.Name,
				Description: encProj.Description,
				Repo: &myawesomelistv1.Repository{
					Hostname: "github.com",
					Owner:    owner,
					Repo:     repo,
				},
			}
		}
		categories[i] = &myawesomelistv1.Category{
			Name:     encCat.Name,
			Projects: projects,
		}
	}

	collection := &myawesomelistv1.Collection{
		Language:   encColl.Language,
		Categories: categories,
	}

	if err := c.d.UpSertCollection(ctx, owner, repo, collection); err != nil {
		slog.Warn("Failed to upsert collection",
			"owner", owner,
			"repo", repo,
			"error", err)
	}

	return collection, nil
}

// GetProjectStats retrieves cached stats or fetches from GitHub and persists them
func (c *GitHubClient) GetProjectStats(ctx context.Context, owner, repo string) (*myawesomelistv1.ProjectsStats, error) {
	stats, err := c.d.GetProjectStats(ctx, owner, repo)
	if err != nil {
		slog.Warn("Failed to query project stats from datastore",
			"owner", owner,
			"repo", repo,
			"error", err)
	}
	if stats != nil {
		return stats, nil
	}

	// Fetch from GitHub
	if err = c.l.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}
	repository, _, err := c.c.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo info for %s/%s: %w", owner, repo, err)
	}

	stats = &myawesomelistv1.ProjectsStats{
		StargazersCount: ptr.To(int64(ptr.Deref(repository.StargazersCount, 0))),
		OpenIssueCount:  ptr.To(int64(ptr.Deref(repository.OpenIssuesCount, 0))),
	}

	// Persist stats
	if err := c.d.UpsertProjectStats(ctx, owner, repo, stats); err != nil {
		slog.Warn("Failed to upsert project stats",
			"owner", owner,
			"repo", repo,
			"error", err)
	}

	return stats, nil
}
