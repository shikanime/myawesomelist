package awesome

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	v1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

type DataStore struct {
	db *sql.DB
}

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
func (ds *DataStore) GetCollection(ctx context.Context, owner, repo string) (*v1.Collection, error) {
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

	var collection v1.Collection
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
func (ds *DataStore) UpSertCollection(ctx context.Context, owner, repo string, collection *v1.Collection) error {
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

// SearchProjects queries stored collections for projects matching the query.
// Matches against project name, description, or URL, optionally restricted to repos.
func (ds *DataStore) SearchProjects(ctx context.Context, q string, limit int32, repos []v1.Repository) ([]*v1.Project, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	if limit <= 0 {
		limit = 50
	}

	// Case-insensitive pattern
	pattern := "%" + strings.ToLower(q) + "%"

	// Build SQL with protobuf JSON field names
	query := `
		SELECT
			p->>'name' AS name,
			p->>'description' AS description,
			p->>'url' AS url,
			NULLIF((p->'stats'->>'stargazers_count'),'')::int AS stargazers,
			NULLIF((p->'stats'->>'open_issue_count'),'')::int AS open_issues
		FROM collections c
		JOIN LATERAL jsonb_array_elements((c.data::jsonb)->'categories') cat ON TRUE
		JOIN LATERAL jsonb_array_elements(cat->'projects') p ON TRUE
		WHERE
			(LOWER(p->>'name') LIKE $1 OR LOWER(p->>'description') LIKE $1 OR LOWER(p->>'url') LIKE $1)
	`
	args := []any{pattern}

	if len(repos) > 0 {
		query += " AND ("
		for i, r := range repos {
			if i > 0 {
				query += " OR "
			}
			query += fmt.Sprintf("(c.owner = $%d AND c.repo = $%d)", len(args)+1, len(args)+2)
			args = append(args, r.Owner, r.Repo)
		}
		query += ")"
	}

	query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := ds.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var results []*v1.Project
	for rows.Next() {
		var name, description, url string
		var stargazers, openIssues sql.NullInt64
		if err := rows.Scan(&name, &description, &url, &stargazers, &openIssues); err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}
		p := &v1.Project{
			Name:        name,
			Description: description,
			Url:         url,
		}
		// Initialize nested stats
		p.Stats = &v1.ProjectsStats{}
		if stargazers.Valid {
			v := int64(stargazers.Int64)
			p.Stats.StargazersCount = &v
		}
		if openIssues.Valid {
			v := int64(openIssues.Int64)
			p.Stats.OpenIssueCount = &v
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search rows error: %w", err)
	}

	slog.Debug("Search results", "query", q, "repos", len(repos), "count", len(results))
	return results, nil
}
