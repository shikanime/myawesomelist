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

type GitHubClientOptions struct {
	token string
}

type GitHubClientOption func(*GitHubClientOptions)

func WithToken(token string) GitHubClientOption {
	return func(o *GitHubClientOptions) { o.token = token }
}

// NewGitHubClient creates a new GitHub client with optional authentication
func NewGitHubClient(ds *DataStore, opts ...GitHubClientOption) *GitHubClient {
	// Use provided datastore as-is; main is responsible for ds.Connect()
	if ds == nil || ds.db == nil {
		slog.Warn("Datastore not configured; disabled")
		ds = &DataStore{db: nil}
	}

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
func (c *GitHubClient) GetCollection(ctx context.Context, owner, repo string, opts ...Option) (Collection, error) {
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
			errors := c.EnrichCollectionWithRepoInfo(ctx, *cachedCollection, opts...)
			if len(errors) > 0 {
				slog.Warn("Encountered errors during enrichment of cached collection",
					"owner", owner,
					"repo", repo,
					"total_errors", len(errors))
			}
		}

		return *cachedCollection, nil
	}

	// Collection not found in datastore or is stale, fetch from GitHub API
	slog.Info("Fetching collection from GitHub API",
		"owner", owner,
		"repo", repo)

	content, err := c.GetReadme(ctx, owner, repo)
	if err != nil {
		return Collection{}, fmt.Errorf("failed to read content for %s/%s: %v", owner, repo, err)
	}

	// Parse using encoding package with embedded options
	collection, err := encoding.UnmarshallCollection(content, options.eopts...)
	if err != nil {
		return Collection{}, fmt.Errorf("failed to parse collection for %s/%s: %v", owner, repo, err)
	}

	categories := make([]Category, len(collection.Categories))
	for i, category := range collection.Categories {
		projects := make([]Project, len(category.Projects))
		for j, encProj := range category.Projects {
			projects[j] = Project{
				Name:        encProj.Name,
				Description: encProj.Description,
				URL:         encProj.URL,
			}
		}
		categories[i] = Category{
			Name:     category.Name,
			Projects: projects,
		}
	}

	enrichedCollection := Collection{
		Language:   collection.Language,
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
func (c *GitHubClient) EnrichProjectWithRepoInfo(ctx context.Context, project *Project, opts ...Option) error {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	if options.includeRepoInfo && strings.Contains(project.URL, "github.com") {
		slog.Debug("Enriching project with GitHub repo info",
			"project", project.Name,
			"url", project.URL)

		owner, repo, err := ExtractGitHubRepoFromURL(project.URL)
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

		project.StargazersCount = repository.StargazersCount
		project.OpenIssueCount = repository.OpenIssuesCount
	} else {
		slog.Debug("Skipping enrichment for project",
			"project", project.Name,
			"include_github_repo_info", options.includeRepoInfo,
			"is_github", strings.Contains(project.URL, "github.com"))
	}

	return nil
}

// EnrichCollectionWithRepoInfo enriches all projects in a collection with GitHub information using parallel processing
func (c *GitHubClient) EnrichCollectionWithRepoInfo(ctx context.Context, collection Collection, opts ...Option) []error {
	var categoryWg sync.WaitGroup
	var errors []error
	for _, category := range collection.Categories {
		categoryWg.Go(func() {
			slog.Debug("Processing category",
				"category", category.Name,
				"projects", len(category.Projects))

			var projectWg sync.WaitGroup
			for _, project := range category.Projects {
				projectWg.Go(func() {
					if err := c.EnrichProjectWithRepoInfo(ctx, &project, opts...); err != nil {
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

// Close closes the GitHub client and its datastore
func (c *GitHubClient) Close() error {
	if c.d != nil {
		return c.d.Close()
	}
	return nil
}
