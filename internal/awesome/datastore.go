package awesome

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// CollectionRecord represents a collection stored in the database
type CollectionRecord struct {
	ID        int       `json:"id"`
	Owner     string    `json:"owner"`
	Repo      string    `json:"repo"`
	Language  string    `json:"language"`
	Data      string    `json:"data"` // JSON-encoded Collection
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DataStore struct {
	db *sql.DB
}

func NewDataStore(db *sql.DB) *DataStore {
	return &DataStore{
		db: db,
	}
}

// Connect verifies the provided database connection is available
func (ds *DataStore) Connect() error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}
	if err := ds.db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	return nil
}

// GetCollection retrieves a collection from the database
func (ds *DataStore) GetCollection(ctx context.Context, owner, repo string) (*Collection, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	query := `
		SELECT language, data, updated_at
		FROM collections
		WHERE owner = $1 AND repo = $2
	`

	var language, data string
	var updatedAt time.Time

	err := ds.db.QueryRowContext(ctx, query, owner, repo).Scan(&language, &data, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}

	var collection Collection
	if err := json.Unmarshal([]byte(data), &collection); err != nil {
		return nil, fmt.Errorf("failed to unmarshal collection data: %w", err)
	}

	slog.Debug("Retrieved collection from database",
		"owner", owner,
		"repo", repo,
		"categories", len(collection.Categories))

	return &collection, nil
}

// UpSertCollection stores a collection in the database
func (ds *DataStore) UpSertCollection(ctx context.Context, owner, repo string, collection Collection) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}

	data, err := json.Marshal(collection)
	if err != nil {
		return fmt.Errorf("failed to marshal collection: %w", err)
	}

	query := `
		INSERT INTO collections (owner, repo, language, data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (owner, repo)
		DO UPDATE SET
			language = EXCLUDED.language,
			data = EXCLUDED.data
	`

	_, err = ds.db.ExecContext(ctx, query, owner, repo, collection.Language, string(data))
	if err != nil {
		return fmt.Errorf("failed to store collection: %w", err)
	}

	slog.Debug("Stored collection in database",
		"owner", owner,
		"repo", repo,
		"categories", len(collection.Categories))

	return nil
}

// Close closes the database connection
func (ds *DataStore) Close() error {
	if ds.db != nil {
		return ds.db.Close()
	}
	return nil
}
