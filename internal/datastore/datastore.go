package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"myawesomelist.shikanime.studio/internal/awesome"
)

// Datastore interface defines the methods for data persistence
type Datastore interface {
	UpsertCollection(ctx context.Context, owner, repo string, collection awesome.Collection) (*CollectionRecord, error)
	GetCollection(ctx context.Context, owner, repo string) (*CollectionRecord, error)
	GetAllCollections(ctx context.Context) ([]CollectionRecord, error)
	DeleteCollection(ctx context.Context, owner, repo string) error
}

// SQLiteDatastore implements the Datastore interface using SQLite
type SQLiteDatastore struct {
	db *sql.DB
}

// OpenSQLiteDatastore creates a new SQLite datastore
func OpenSQLiteDatastore(dbPath string) (*SQLiteDatastore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return &SQLiteDatastore{db: db}, nil
}

// Init initializes the datastore by enabling foreign keys
func (s *SQLiteDatastore) Init() error {
	if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	return nil
}

// Migrate creates the database schema
func (s *SQLiteDatastore) Migrate() error {
	_, err := s.db.Exec(Schema)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}
	return nil
}

// Close closes the database connection
func (s *SQLiteDatastore) Close() error {
	return s.db.Close()
}

// UpsertCollection stores a collection and its categories/projects in the database
func (s *SQLiteDatastore) UpsertCollection(ctx context.Context, owner, repo string, collection awesome.Collection) (*CollectionRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Insert or update collection
	var collectionID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO collections (language, owner, repo, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(owner, repo) DO UPDATE SET
			language = excluded.language,
			updated_at = excluded.updated_at
		RETURNING id
	`, collection.Language, owner, repo, now, now).Scan(&collectionID)

	if err != nil {
		return nil, fmt.Errorf("failed to insert/update collection: %w", err)
	}

	// Delete existing categories and projects (cascade will handle projects)
	_, err = tx.ExecContext(ctx, "DELETE FROM categories WHERE collection_id = ?", collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete existing categories: %w", err)
	}

	// Store categories and projects
	var categoryRecords []CategoryRecord
	for _, category := range collection.Categories {
		categoryID, err := s.storeCategory(ctx, tx, collectionID, category, now)
		if err != nil {
			return nil, fmt.Errorf("failed to store category %s: %w", category.Name, err)
		}

		projectRecords, err := s.storeProjects(ctx, tx, categoryID, category.Projects, now)
		if err != nil {
			return nil, fmt.Errorf("failed to store projects for category %s: %w", category.Name, err)
		}

		categoryRecords = append(categoryRecords, CategoryRecord{
			ID:           categoryID,
			CollectionID: collectionID,
			Name:         category.Name,
			Projects:     projectRecords,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &CollectionRecord{
		ID:         collectionID,
		Language:   collection.Language,
		Owner:      owner,
		Repo:       repo,
		Categories: categoryRecords,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// storeCategory stores a single category
func (s *SQLiteDatastore) storeCategory(ctx context.Context, tx *sql.Tx, collectionID int64, category awesome.Category, now time.Time) (int64, error) {
	var categoryID int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO categories (collection_id, name, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		RETURNING id
	`, collectionID, category.Name, now, now).Scan(&categoryID)

	if err != nil {
		return 0, fmt.Errorf("failed to insert category: %w", err)
	}

	return categoryID, nil
}

// storeProjects stores multiple projects for a category
func (s *SQLiteDatastore) storeProjects(ctx context.Context, tx *sql.Tx, categoryID int64, projects []awesome.Project, now time.Time) ([]ProjectRecord, error) {
	var projectRecords []ProjectRecord

	for _, project := range projects {
		var projectID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO projects (category_id, name, description, url, stargazers_count, open_issue_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			RETURNING id
		`, categoryID, project.Name, project.Description, project.URL, project.StargazersCount, project.OpenIssueCount, now, now).Scan(&projectID)

		if err != nil {
			return nil, fmt.Errorf("failed to insert project %s: %w", project.Name, err)
		}

		projectRecords = append(projectRecords, ProjectRecord{
			ID:              projectID,
			CategoryID:      categoryID,
			Name:            project.Name,
			Description:     project.Description,
			URL:             project.URL,
			StargazersCount: project.StargazersCount,
			OpenIssueCount:  project.OpenIssueCount,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}

	return projectRecords, nil
}

// GetCollection retrieves a collection by owner and repo
func (s *SQLiteDatastore) GetCollection(ctx context.Context, owner, repo string) (*CollectionRecord, error) {
	var collection CollectionRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT id, language, owner, repo, created_at, updated_at
		FROM collections
		WHERE owner = ? AND repo = ?
	`, owner, repo).Scan(&collection.ID, &collection.Language, &collection.Owner, &collection.Repo, &collection.CreatedAt, &collection.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	// Load categories
	categories, err := s.getCategoriesForCollection(ctx, collection.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load categories: %w", err)
	}
	collection.Categories = categories

	return &collection, nil
}

// GetAllCollections retrieves all collections
func (s *SQLiteDatastore) GetAllCollections(ctx context.Context) ([]CollectionRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, language, owner, repo, created_at, updated_at
		FROM collections
		ORDER BY language, owner, repo
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query collections: %w", err)
	}
	defer rows.Close()

	var collections []CollectionRecord
	for rows.Next() {
		var collection CollectionRecord
		err := rows.Scan(&collection.ID, &collection.Language, &collection.Owner, &collection.Repo, &collection.CreatedAt, &collection.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan collection: %w", err)
		}

		// Load categories for each collection
		categories, err := s.getCategoriesForCollection(ctx, collection.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load categories for collection %d: %w", collection.ID, err)
		}
		collection.Categories = categories

		collections = append(collections, collection)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating collections: %w", err)
	}

	return collections, nil
}

// getCategoriesForCollection retrieves categories for a specific collection
func (s *SQLiteDatastore) getCategoriesForCollection(ctx context.Context, collectionID int64) ([]CategoryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, collection_id, name, created_at, updated_at
		FROM categories
		WHERE collection_id = ?
		ORDER BY name
	`, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var categories []CategoryRecord
	for rows.Next() {
		var category CategoryRecord
		err := rows.Scan(&category.ID, &category.CollectionID, &category.Name, &category.CreatedAt, &category.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		// Load projects for each category
		projects, err := s.getProjectsForCategory(ctx, category.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load projects for category %d: %w", category.ID, err)
		}
		category.Projects = projects

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	return categories, nil
}

// getProjectsForCategory retrieves projects for a specific category
func (s *SQLiteDatastore) getProjectsForCategory(ctx context.Context, categoryID int64) ([]ProjectRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, category_id, name, description, url, stargazers_count, open_issue_count, created_at, updated_at
		FROM projects
		WHERE category_id = ?
		ORDER BY name
	`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to query projects: %w", err)
	}
	defer rows.Close()

	var projects []ProjectRecord
	for rows.Next() {
		var project ProjectRecord
		err := rows.Scan(&project.ID, &project.CategoryID, &project.Name, &project.Description, &project.URL, &project.StargazersCount, &project.OpenIssueCount, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating projects: %w", err)
	}

	return projects, nil
}

// DeleteCollection deletes a collection and all its related data
func (s *SQLiteDatastore) DeleteCollection(ctx context.Context, owner, repo string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM collections WHERE owner = ? AND repo = ?", owner, repo)
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("collection not found")
	}

	return nil
}
