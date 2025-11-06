package awesome

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v75/github"
	"golang.org/x/time/rate"
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
			"rate", "1 request/second",
			"burst", 10)
	} else {
		limiter = rate.NewLimiter(rate.Every(time.Hour), 60)
		slog.Info("Created unauthenticated GitHub rate limiter",
			"rate", "1 request/minute",
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

		// If we need repo info and it's not already enriched, enrich it
		if options.includeRepoInfo {
			errors := c.EnrichCollectionWithRepoInfo(ctx, cachedCollection, opts...)
			if len(errors) > 0 {
				slog.Warn("Encountered errors during enrichment of cached collection",
					"owner", owner,
					"repo", repo,
					"total_errors", len(errors))
			}
		}

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
				Url:         encProj.URL,
			}
		}
		categories[i] = &myawesomelistv1.Category{
			Name:     encCat.Name,
			Projects: projects,
		}
	}

	enrichedCollection := &myawesomelistv1.Collection{
		Language:   encColl.Language,
		Categories: categories,
	}

	// Enrich with repo info if requested
	errors := c.EnrichCollectionWithRepoInfo(ctx, enrichedCollection, opts...)
	if len(errors) > 0 {
		slog.Warn("Encountered errors during enrichment",
			"owner", owner,
			"repo", repo,
			"total_errors", len(errors))
	}

	// Store the collection in the datastore for future use
	if err := c.d.UpSertCollection(ctx, owner, repo, enrichedCollection); err != nil {
		slog.Warn("Failed to store collection in datastore",
			"owner", owner,
			"repo", repo,
			"error", err)
		// Don't return error here, as the main operation succeeded
	}

	return enrichedCollection, nil
}

// EnrichProjectWithRepoInfo enriches a single project with GitHub repository information
func (c *GitHubClient) EnrichProjectWithRepoInfo(ctx context.Context, project *myawesomelistv1.Project, opts ...Option) error {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	if options.includeRepoInfo && strings.Contains(project.Url, "github.com") {
		slog.Debug("Enriching project with GitHub repo info",
			"project", project.Name,
			"url", project.Url)

		owner, repo, err := ExtractGitHubRepoFromURL(project.Url)
		if err != nil {
			slog.Error("Failed to extract repo info for project",
				"project", project.Name,
				"error", err)
			return fmt.Errorf("failed to extract repo info for project %s: %w", project.Name, err)
		}

		slog.Debug("Fetching GitHub data",
			"owner", owner,
			"repo", repo,
			"project", project.Name)

		// Get repository information (includes stargazer count and open issues)
		if err = c.l.Wait(ctx); err != nil {
			return fmt.Errorf("rate limiter wait failed: %w", err)
		}
		repository, _, err := c.c.Repositories.Get(ctx, owner, repo)
		if err != nil {
			slog.Error("Failed to get repo info from GitHub API",
				"project", project.Name,
				"owner", owner,
				"repo", repo,
				"error", err)
			return fmt.Errorf("failed to get repo info for project %s: %w", project.Name, err)
		}

		if project.Stats == nil {
			project.Stats = &myawesomelistv1.ProjectsStats{}
		}
		if repository.StargazersCount != nil {
			v := int64(*repository.StargazersCount)
			project.Stats.StargazersCount = &v
		}
		if repository.OpenIssuesCount != nil {
			v := int64(*repository.OpenIssuesCount)
			project.Stats.OpenIssueCount = &v
		}
	} else {
		slog.Debug("Skipping enrichment for project",
			"project", project.Name,
			"include_github_repo_info", options.includeRepoInfo,
			"is_github", strings.Contains(project.Url, "github.com"))
	}

	return nil
}

// EnrichCollectionWithRepoInfo enriches all projects in a collection with GitHub information using parallel processing
func (c *GitHubClient) EnrichCollectionWithRepoInfo(ctx context.Context, collection *myawesomelistv1.Collection, opts ...Option) []error {
	var categoryWg sync.WaitGroup
	var errors []error
	for _, category := range collection.Categories {
		categoryWg.Go(func() {
			slog.Debug("Processing category",
				"category", category.Name,
				"projects", len(category.Projects))

			var projectWg sync.WaitGroup
			for _, project := range category.Projects {
				p := project
				projectWg.Go(func() {
					if err := c.EnrichProjectWithRepoInfo(ctx, p, opts...); err != nil {
						slog.Warn("Error processing project",
							"project", project.Name,
							"category", category.Name,
							"error", err)
						errors = append(errors, err)
					}
				})
			}
			projectWg.Wait()
		})
	}
	categoryWg.Wait()

	return errors
}

// SearchProjects performs SQL-backed search over stored collections.
func (c *GitHubClient) SearchProjects(ctx context.Context, q string, limit int32, repos []myawesomelistv1.Repository) ([]*myawesomelistv1.Project, error) {
	return c.d.SearchProjects(ctx, q, limit, repos)
}

// Add a readiness check that verifies the datastore is reachable
func (c *GitHubClient) Ping(ctx context.Context) error {
	if c.d == nil {
		return fmt.Errorf("datastore not configured")
	}
	return c.d.Ping(ctx)
}
