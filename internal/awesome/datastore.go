package awesome

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	sqlx "myawesomelist.shikanime.studio/internal/sqlx"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// DataStore wraps a SQL database and provides typed operations for collections.
type DataStore struct {
	db *sql.DB
}

// NewDataStore constructs a DataStore using the provided sql.DB connection.
func NewDataStore(db *sql.DB) *DataStore {
	return &DataStore{
		db: db,
	}
}

// Ping verifies the provided database connection is available
func (ds *DataStore) Ping(ctx context.Context) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}
	if err := ds.db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	return nil
}

// GetCollection retrieves a collection from the database
func (ds *DataStore) GetCollection(ctx context.Context, owner, repo string) (*myawesomelistv1.Collection, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	stmt, args, err := ds.PrepareGetCollection(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to build get collection query: %w", err)
	}
	defer stmt.Close()

	var language, data string
	var updatedAt time.Time

	if err = stmt.QueryRowContext(ctx, args...).Scan(&language, &data, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}

	var collection myawesomelistv1.Collection
	if err := json.Unmarshal([]byte(data), &collection); err != nil {
		return nil, fmt.Errorf("failed to unmarshal collection data: %w", err)
	}

	slog.Debug("Retrieved collection from database",
		"owner", owner,
		"repo", repo,
		"categories", len(collection.Categories))

	return &collection, nil
}

// PrepareGetCollection renders and prepares the SQL statement to fetch a collection.
func (ds *DataStore) PrepareGetCollection(ctx context.Context, owner, repo string) (*sql.Stmt, []any, error) {
	query, args, err := sqlx.GetCollectionQuery(owner, repo)
	if err != nil {
		return nil, nil, err
	}
	stmt, err := ds.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	return stmt, args, nil
}

// UpSertCollection stores a collection in the database
func (ds *DataStore) UpSertCollection(ctx context.Context, owner, repo string, collection *myawesomelistv1.Collection) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}

	stmt, args, err := ds.PrepareUpsertCollection(ctx, owner, repo, collection)
	if err != nil {
		return fmt.Errorf("failed to build upsert collection query: %w", err)
	}
	defer stmt.Close()

	if _, err = stmt.ExecContext(ctx, args...); err != nil {
		return fmt.Errorf("failed to store collection: %w", err)
	}

	slog.Debug("Stored collection in database",
		"owner", owner,
		"repo", repo,
		"categories", len(collection.Categories))

	return nil
}

// PrepareUpsertCollection renders and prepares the SQL statement to upsert a collection.
func (ds *DataStore) PrepareUpsertCollection(ctx context.Context, owner, repo string, collection *myawesomelistv1.Collection) (*sql.Stmt, []any, error) {
	data, err := json.Marshal(collection)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal collection: %w", err)
	}
	query, args, err := sqlx.UpsertCollectionQuery(owner, repo, collection.Language, string(data))
	if err != nil {
		return nil, nil, err
	}
	stmt, err := ds.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	return stmt, args, nil
}

// SearchProjects executes a datastore-backed search across repositories.
func (ds *DataStore) SearchProjects(ctx context.Context, q string, limit int32, repos []*myawesomelistv1.Repository) ([]*myawesomelistv1.Project, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	if limit <= 0 {
		limit = 50
	}

	stmt, args, err := ds.PrepareSearchProjects(ctx, q, repos, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to render search query: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var projects []*myawesomelistv1.Project
	for rows.Next() {
		var project myawesomelistv1.Project
		err := rows.Scan(&project)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}

		projects = append(projects, &project)
	}

	return projects, nil
}

// PrepareSearchProjects renders and prepares the SQL statement for project search.
func (ds *DataStore) PrepareSearchProjects(ctx context.Context, q string, repos []*myawesomelistv1.Repository, limit int32) (*sql.Stmt, []any, error) {
	query, args, err := sqlx.SearchProjectsQuery(q, repos, limit)
	if err != nil {
		return nil, nil, err
	}
	stmt, err := ds.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	return stmt, args, nil
}

// Close closes the database connection
func (ds *DataStore) Close() error {
	if ds.db != nil {
		return ds.db.Close()
	}
	return nil
}
