package awesome

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"k8s.io/utils/ptr"
	sqlx "myawesomelist.shikanime.studio/internal/sqlx"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// DataStore wraps a SQL database and provides typed operations for cols.
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

	tx, err := ds.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	// Get collection id and language
	q, args, err := sqlx.GetCollectionQuery(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to build get collection query: %w", err)
	}

	var colID int64
	col := &myawesomelistv1.Collection{}
	if err = tx.QueryRowContext(ctx, q, args...).Scan(&colID, &col.Language); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}

	// Load categories for the collection
	categoriesQuery, catArgs, err := sqlx.GetCategoriesQuery(colID)
	if err != nil {
		return nil, fmt.Errorf("failed to build get categories query: %w", err)
	}
	categoriesRows, err := tx.QueryContext(ctx, categoriesQuery, catArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer categoriesRows.Close()

	for categoriesRows.Next() {
		var categoryID int64
		category := &myawesomelistv1.Category{}
		if err := categoriesRows.Scan(&categoryID, &category.Name); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		// Load projects for the category
		projectsQuery, projArgs, err := sqlx.GetProjectsQuery(categoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to build get projects query: %w", err)
		}
		projectsRows, err := tx.QueryContext(ctx, projectsQuery, projArgs...)
		if err != nil {
			return nil, fmt.Errorf("failed to query projects: %w", err)
		}

		for projectsRows.Next() {
			p := &myawesomelistv1.Project{Repo: &myawesomelistv1.Repository{}}
			if err := projectsRows.Scan(
				&p.Name,
				&p.Description,
				&p.Repo.Hostname,
				&p.Repo.Owner,
				&p.Repo.Repo,
			); err != nil {
				projectsRows.Close()
				return nil, fmt.Errorf("failed to scan project: %w", err)
			}
			category.Projects = append(category.Projects, p)
		}
		projectsRows.Close()

		col.Categories = append(col.Categories, category)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit tx: %w", err)
	}

	slog.Debug("Retrieved collection from database",
		"hostname", repo.Hostname,
		"owner", repo.Owner,
		"repo", repo.Repo,
		"categories", len(col.Categories))

	return col, nil
}

// PrepareGetCollection renders and prepares the SQL statement to fetch a collection.
func (ds *DataStore) PrepareGetCollection(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*sql.Stmt, []any, error) {
	q, args, err := sqlx.GetCollectionQuery(repo)
	if err != nil {
		return nil, nil, err
	}
	stmt, err := ds.db.PrepareContext(ctx, q)
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

	for _, category := range col.Categories {
		categoryQuery, categoryArgs, categoryErr := sqlx.UpSertCategoryQuery(collectionID, category)
		if categoryErr != nil {
			return fmt.Errorf("failed to build upsert category query: %w", categoryErr)
		}
		var categoryID int64
		if categoryErr = tx.QueryRowContext(ctx, categoryQuery, categoryArgs...).Scan(&categoryID); categoryErr != nil {
			return fmt.Errorf("failed to upsert category: %w", categoryErr)
		}

		for _, p := range category.Projects {
			projectQuery, projectArgs, projectErr := sqlx.UpSertProjectQuery(categoryID, p)
			if projectErr != nil {
				return fmt.Errorf("failed to build upsert project query: %w", projectErr)
			}
			if _, projectErr = tx.ExecContext(ctx, projectQuery, projectArgs...); projectErr != nil {
				return fmt.Errorf("failed to upsert project: %w", projectErr)
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
	q, args, err := sqlx.UpSertCollectionQuery(repo, col)
	if err != nil {
		return nil, nil, err
	}
	stmt, err := ds.db.PrepareContext(ctx, q)
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

	sqlQuery, args, err := sqlx.SearchProjectsQuery(q, repos, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to render search query: %w", err)
	}

	rows, err := ds.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var projects []*myawesomelistv1.Project
	for rows.Next() {
		project := &myawesomelistv1.Project{Repo: &myawesomelistv1.Repository{}}
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
	query string,
	repos []*myawesomelistv1.Repository,
	limit uint32,
) (*sql.Stmt, []any, error) {
	q, args, err := sqlx.SearchProjectsQuery(query, repos, limit)
	if err != nil {
		return nil, nil, err
	}
	stmt, err := ds.db.PrepareContext(ctx, q)
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

	sqlQuery, args, err := sqlx.GetProjectStatsQuery(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to build get project stats query: %w", err)
	}

	var stargazers sql.NullInt32
	var openIssues sql.NullInt32
	var updatedAt time.Time

	if err = ds.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&stargazers, &openIssues, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query project stats: %w", err)
	}

	stats := &myawesomelistv1.ProjectStats{}
	if stargazers.Valid {
		stats.StargazersCount = ptr.To(uint32(stargazers.Int32))
	}
	if openIssues.Valid {
		stats.OpenIssueCount = ptr.To(uint32(openIssues.Int32))
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

	sqlQuery, args, err := sqlx.UpSertProjectStatsQuery(repo, stats)
	if err != nil {
		return fmt.Errorf("failed to build upsert project stats query: %w", err)
	}

	if _, err = ds.db.ExecContext(ctx, sqlQuery, args...); err != nil {
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
	q, args, err := sqlx.GetProjectStatsQuery(repo)
	if err != nil {
		return nil, nil, err
	}
	stmt, err := ds.db.PrepareContext(ctx, q)
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
	q, args, err := sqlx.UpSertProjectStatsQuery(repo, stats)
	if err != nil {
		return nil, nil, err
	}
	stmt, err := ds.db.PrepareContext(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	return stmt, args, nil
}
