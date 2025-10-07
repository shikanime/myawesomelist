package datastore

import (
	"context"
	"fmt"
	"log"
	"time"

	"myawesomelist.shikanime.studio/internal/awesome"
)

// Service provides high-level operations combining awesome and datastore functionality
type Service struct {
	datastore    Datastore
	githubClient *awesome.GitHubClient
}

// NewService creates a new datastore service
func NewService(datastore Datastore, githubClient *awesome.GitHubClient) *Service {
	return &Service{
		datastore:    datastore,
		githubClient: githubClient,
	}
}

// RefreshCollections fetches fresh data from GitHub and stores it in the datastore
func (s *Service) RefreshCollections(ctx context.Context, repos []awesome.GitHubRepoConfig) error {
	for _, repo := range repos {
		log.Printf("Refreshing collection for %s/%s", repo.Owner, repo.Repo)

		// Fetch fresh data from GitHub
		collection, err := s.githubClient.GetCollection(ctx, repo.Owner, repo.Repo, repo.Options...)
		if err != nil {
			log.Printf("Failed to fetch collection for %s/%s: %v", repo.Owner, repo.Repo, err)
			continue
		}

		// Store in datastore
		_, err = s.datastore.UpsertCollection(ctx, repo.Owner, repo.Repo, collection)
		if err != nil {
			log.Printf("Failed to store collection for %s/%s: %v", repo.Owner, repo.Repo, err)
			continue
		}

		log.Printf("Successfully refreshed collection for %s/%s", repo.Owner, repo.Repo)
	}

	return nil
}

// GetCollections retrieves collections from the datastore, with optional refresh
func (s *Service) GetCollections(ctx context.Context, maxAge time.Duration) ([]CollectionRecord, error) {
	collections, err := s.datastore.GetAllCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get collections from datastore: %w", err)
	}

	// Check if we need to refresh based on age
	needsRefresh := len(collections) == 0
	if !needsRefresh && maxAge > 0 {
		for _, collection := range collections {
			if time.Since(collection.UpdatedAt) > maxAge {
				needsRefresh = true
				break
			}
		}
	}

	// Refresh if needed
	if needsRefresh {
		log.Printf("Collections are stale or missing, refreshing from GitHub...")
		err := s.RefreshCollections(ctx, awesome.DefaultGitHubRepos)
		if err != nil {
			log.Printf("Failed to refresh collections: %v", err)
			// Return existing data if refresh fails
			return collections, nil
		}

		// Fetch updated collections
		collections, err = s.datastore.GetAllCollections(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get refreshed collections: %w", err)
		}
	}

	return collections, nil
}

// GetCollection retrieves a specific collection, with optional refresh
func (s *Service) GetCollection(ctx context.Context, owner, repo string, maxAge time.Duration) (*CollectionRecord, error) {
	collection, err := s.datastore.GetCollection(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection from datastore: %w", err)
	}

	// Check if we need to refresh
	needsRefresh := collection == nil
	if !needsRefresh && maxAge > 0 && time.Since(collection.UpdatedAt) > maxAge {
		needsRefresh = true
	}

	// Refresh if needed
	if needsRefresh {
		log.Printf("Collection %s/%s is stale or missing, refreshing from GitHub...", owner, repo)

		// Find the repo config
		var repoConfig *awesome.GitHubRepoConfig
		for _, config := range awesome.DefaultGitHubRepos {
			if config.Owner == owner && config.Repo == repo {
				repoConfig = &config
				break
			}
		}

		if repoConfig == nil {
			return nil, fmt.Errorf("no configuration found for repository %s/%s", owner, repo)
		}

		// Fetch fresh data
		awesomeCollection, err := s.githubClient.GetCollection(ctx, owner, repo, repoConfig.Options...)
		if err != nil {
			log.Printf("Failed to fetch collection for %s/%s: %v", owner, repo, err)
			// Return existing data if refresh fails
			return collection, nil
		}

		// Store updated data
		collection, err = s.datastore.UpsertCollection(ctx, owner, repo, awesomeCollection)
		if err != nil {
			return nil, fmt.Errorf("failed to store refreshed collection: %w", err)
		}
	}

	return collection, nil
}
