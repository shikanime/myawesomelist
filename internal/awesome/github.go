package awesome

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
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
	client  *github.Client
	limiter *rate.Limiter
}

// NewGitHubClient creates a new GitHub client with optional authentication
func NewGitHubClient() *GitHubClient {
	// Check for GitHub token in environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		slog.Info("Using authenticated GitHub client")
		return &GitHubClient{
			client:  github.NewClient(nil).WithAuthToken(token),
			limiter: NewGitHubLimiter(true),
		}
	} else {
		slog.Warn("Using unauthenticated GitHub client (rate limited)")
		return &GitHubClient{
			client:  github.NewClient(nil),
			limiter: NewGitHubLimiter(false),
		}
	}
}

// GetReadme creates a reader for the README.md file of the specified repository
func (c *GitHubClient) GetReadme(ctx context.Context, owner string, repo string) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}
	file, _, _, err := c.client.Repositories.GetContents(
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
	errors := c.EnrichCollectionWithRepoInfo(ctx, enrichedCollection, opts...)
	if len(errors) > 0 {
		slog.Warn("Encountered errors during enrichment",
			"owner", owner,
			"repo", repo,
			"total_errors", len(errors))
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
		if err = c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limiter wait failed: %w", err)
		}
		repository, _, err := c.client.Repositories.Get(ctx, owner, repo)
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
