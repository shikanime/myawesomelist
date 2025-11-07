package awesome

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"k8s.io/utils/ptr"
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
func (ds *DataStore) GetCollection(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*myawesomelistv1.Collection, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	stmt, args, err := ds.PrepareGetCollection(ctx, repo)
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
		"hostname", repo.Hostname,
		"owner", repo.Owner,
		"repo", repo.Repo,
		"categories", len(collection.Categories))

	return &collection, nil
}

// PrepareGetCollection renders and prepares the SQL statement to fetch a collection.
func (ds *DataStore) PrepareGetCollection(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*sql.Stmt, []any, error) {
	query, args, err := sqlx.GetCollectionQuery(repo)
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
func (ds *DataStore) UpSertCollection(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
	col *myawesomelistv1.Collection,
) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}

	tx, err := ds.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	q, args, err := sqlx.UpSertCollectionQuery(repo, col)
	if err != nil {
		return fmt.Errorf("failed to build upsert collection query: %w", err)
	}
	var collectionID int64
	if err = tx.QueryRowContext(ctx, q, args...).Scan(&collectionID); err != nil {
		return fmt.Errorf("failed to store collection: %w", err)
	}

	for _, cat := range col.Categories {
		catQuery, catArgs, catErr := sqlx.UpSertCategoryQuery(collectionID, cat)
		if catErr != nil {
			return fmt.Errorf("failed to build upsert category query: %w", catErr)
		}
		var categoryID int64
		if catErr = tx.QueryRowContext(ctx, catQuery, catArgs...).Scan(&categoryID); catErr != nil {
			return fmt.Errorf("failed to upsert category: %w", catErr)
		}

		for _, p := range cat.Projects {
			pq, pargs, perr := sqlx.UpSertProjectQuery(categoryID, p)
			if perr != nil {
				return fmt.Errorf("failed to build upsert project query: %w", perr)
			}
			if _, perr = tx.ExecContext(ctx, pq, pargs...); perr != nil {
				return fmt.Errorf("failed to upsert project: %w", perr)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	slog.Debug("Stored collection and projects in database",
		"hostname", repo.Hostname,
		"owner", repo.Owner,
		"repo", repo.Repo,
		"categories", len(col.Categories))

	return nil
}

// PrepareUpSertCollection renders and prepares the SQL statement to upsert a collection.
func (ds *DataStore) PrepareUpSertCollection(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
	col *myawesomelistv1.Collection,
) (*sql.Stmt, []any, error) {
	query, args, err := sqlx.UpSertCollectionQuery(repo, col)
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
func (ds *DataStore) SearchProjects(
	ctx context.Context,
	q string,
	limit uint32,
	repos []*myawesomelistv1.Repository,
) ([]*myawesomelistv1.Project, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
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
		project := &myawesomelistv1.Project{
			Repo: &myawesomelistv1.Repository{},
		}
		err := rows.Scan(
			&project.Name,
			&project.Description,
			&project.Repo.Hostname,
			&project.Repo.Owner,
			&project.Repo.Repo,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// PrepareSearchProjects renders and prepares the SQL statement for project search.
func (ds *DataStore) PrepareSearchProjects(
	ctx context.Context,
	q string,
	repos []*myawesomelistv1.Repository,
	limit uint32,
) (*sql.Stmt, []any, error) {
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

// GetProjectStats retrieves project stats from the datastore
func (ds *DataStore) GetProjectStats(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*myawesomelistv1.ProjectStats, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	stmt, args, err := ds.PrepareGetProjectStats(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to build get project stats query: %w", err)
	}
	defer stmt.Close()

	var stargazers sql.NullInt32
	var openIssues sql.NullInt32
	var updatedAt time.Time

	if err = stmt.QueryRowContext(ctx, args...).Scan(&stargazers, &openIssues, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query project stats: %w", err)
	}

	stats := &myawesomelistv1.ProjectStats{}
	if stargazers.Valid {
		stats.StargazersCount = ptr.To(stargazers.Int32)
	}
	if openIssues.Valid {
		stats.OpenIssueCount = ptr.To(openIssues.Int32)
	}
	return stats, nil
}

// UpSertProjectStats stores project stats in the datastore
func (ds *DataStore) UpSertProjectStats(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
	stats *myawesomelistv1.ProjectStats,
) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}

	stmt, args, err := ds.PrepareUpSertProjectStats(ctx, repo, stats)
	if err != nil {
		return fmt.Errorf("failed to build upsert project stats query: %w", err)
	}
	defer stmt.Close()

	if _, err = stmt.ExecContext(ctx, args...); err != nil {
		return fmt.Errorf("failed to store project stats: %w", err)
	}

	slog.Debug("Stored project stats in database",
		"hostname", repo.Hostname,
		"owner", repo.Owner,
		"repo", repo.Repo,
		"stargazers_count", stats.StargazersCount,
		"open_issue_count", stats.OpenIssueCount)

	return nil
}

// PrepareGetProjectStats renders and prepares the SQL statement to fetch project stats.
func (ds *DataStore) PrepareGetProjectStats(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*sql.Stmt, []any, error) {
	query, args, err := sqlx.GetProjectStatsQuery(repo)
	if err != nil {
		return nil, nil, err
	}
	stmt, err := ds.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	return stmt, args, nil
}

// PrepareUpSertProjectStats renders and prepares the SQL statement to upsert project stats.
func (ds *DataStore) PrepareUpSertProjectStats(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
	stats *myawesomelistv1.ProjectStats,
) (*sql.Stmt, []any, error) {
	query, args, err := sqlx.UpSertProjectStatsQuery(repo, stats)
	if err != nil {
		return nil, nil, err
	}
	stmt, err := ds.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	return stmt, args, nil
}
