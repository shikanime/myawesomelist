package awesome

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/google/go-github/v75/github"
	"myawesomelist.shikanime.studio/internal/encoding"
)

type GetContentConfig struct {
	Owner string
	Repo  string
}

// ReadContent creates a reader for the README.md file of the specified repository
func ReadContent(ctx context.Context, client *github.Client, config GetContentConfig) ([]byte, error) {
	file, _, _, err := client.Repositories.GetContents(
		ctx,
		config.Owner,
		config.Repo,
		"README.md",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %v", err)
	}
	content, err := base64.StdEncoding.DecodeString(*file.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode content: %v", err)
	}
	return content, nil
}

type GetCategoriesConfig struct {
	ContentConfig GetContentConfig
	StartSection  string
}

// GetCategories parses categories from a repository using the provided configuration
func GetCategories(ctx context.Context, client *github.Client, config GetCategoriesConfig) ([]encoding.Category, error) {
	content, err := ReadContent(ctx, client, config.ContentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %v", err)
	}
	return encoding.UnmarshallCategories(content, config.StartSection)
}

// Collection represents a collection of projects grouped by language
type Collection struct {
	Language   string
	Categories []encoding.Category
}

// GetCollectionsConfig represents configuration for a data source
type GetCollectionsConfig struct {
	Language         string
	CategoriesConfig GetCategoriesConfig
}

// GetCollections fetches project collections from multiple awesome repositories
func GetCollections(ctx context.Context, client *github.Client, configs []GetCollectionsConfig) ([]Collection, error) {
	var collections []Collection

	for _, cfg := range configs {
		categories, err := GetCategories(ctx, client, cfg.CategoriesConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s categories: %v", cfg.Language, err)
		}
		log.Printf("Loaded %d %s categories", len(categories), cfg.Language)
		collections = append(collections, Collection{
			Language:   cfg.Language,
			Categories: categories,
		})
	}

	return collections, nil
}
