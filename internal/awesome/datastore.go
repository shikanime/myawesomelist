package awesome

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
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
	var updatedAt time.Time
	if err = tx.QueryRowContext(ctx, q, args...).Scan(&colID, &col.Language, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}
	col.UpdatedAt = timestamppb.New(updatedAt)

	// Load categories for the collection
	categoriesQuery, err := sqlx.GetCategoriesQuery()
	if err != nil {
		return nil, fmt.Errorf("failed to build get categories query: %w", err)
	}
	categoriesRows, err := tx.QueryContext(ctx, categoriesQuery, sqlx.GetCategoriesArgs(colID)...)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer categoriesRows.Close()

	// Prepare projects query once, reuse for each category
	projectsQuery, err := sqlx.GetProjectsQuery()
	if err != nil {
		return nil, fmt.Errorf("failed to build get projects query: %w", err)
	}
	projectsStmt, err := tx.PrepareContext(ctx, projectsQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare projects query: %w", err)
	}
	defer projectsStmt.Close()

	for categoriesRows.Next() {
		var categoryID int64
		category := &myawesomelistv1.Category{}
		var categoryUpdatedAt time.Time
		if err := categoriesRows.Scan(&categoryID, &category.Name, &categoryUpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		category.UpdatedAt = timestamppb.New(categoryUpdatedAt)

		projectsRows, err := projectsStmt.QueryContext(ctx, sqlx.GetProjectsArgs(categoryID)...)
		if err != nil {
			return nil, fmt.Errorf("failed to query projects: %w", err)
		}
		defer projectsRows.Close()

		for projectsRows.Next() {
			p := &myawesomelistv1.Project{Repo: &myawesomelistv1.Repository{}}
			var projectUpdatedAt time.Time
			if err := projectsRows.Scan(
				&p.Name,
				&p.Description,
				&p.Repo.Hostname,
				&p.Repo.Owner,
				&p.Repo.Repo,
				&projectUpdatedAt,
			); err != nil {
				projectsRows.Close()
				return nil, fmt.Errorf("failed to scan project: %w", err)
			}
			p.UpdatedAt = timestamppb.New(projectUpdatedAt)
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

	q, err := sqlx.UpSertCollectionQuery()
	if err != nil {
		return fmt.Errorf("failed to build upsert collection query: %w", err)
	}
	var colID int64
	if err = tx.QueryRowContext(ctx, q, sqlx.UpSertCollectionArgs(repo, col)...).Scan(&colID); err != nil {
		return fmt.Errorf("failed to store collection: %w", err)
	}

	projectQuery, err := sqlx.UpSertProjectQuery()
	if err != nil {
		return fmt.Errorf("failed to build upsert project query: %w", err)
	}
	projectStmt, err := tx.PrepareContext(ctx, projectQuery)
	if err != nil {
		return fmt.Errorf("failed to prepare upsert project statement: %w", err)
	}
	defer projectStmt.Close()

	categoryQuery, err := sqlx.UpSertCategoryQuery()
	if err != nil {
		return fmt.Errorf("failed to build upsert category query: %w", err)
	}
	categoryStmt, err := tx.PrepareContext(ctx, categoryQuery)
	if err != nil {
		return fmt.Errorf("failed to prepare upsert category statement: %w", err)
	}
	defer categoryStmt.Close()

	for _, category := range col.Categories {
		var categoryID int64
		if categoryErr := categoryStmt.QueryRowContext(ctx, sqlx.UpSertCategoryArgs(colID, category)...).Scan(&categoryID); categoryErr != nil {
			return fmt.Errorf("failed to upsert category: %w", categoryErr)
		}

		for _, p := range category.Projects {
			if _, projectErr := projectStmt.ExecContext(ctx, sqlx.UpSertProjectArgs(categoryID, p)...); projectErr != nil {
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

	sqlQuery, err := sqlx.SearchProjectsQuery(q, repos, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to render search query: %w", err)
	}

	rows, err := ds.db.QueryContext(ctx, sqlQuery, sqlx.SearchProjectsArgs(q, repos, limit)...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var projects []*myawesomelistv1.Project
	for rows.Next() {
		project := &myawesomelistv1.Project{Repo: &myawesomelistv1.Repository{}}
		var updatedAt time.Time
		err := rows.Scan(
			&project.Name,
			&project.Description,
			&project.Repo.Hostname,
			&project.Repo.Owner,
			&project.Repo.Repo,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		project.UpdatedAt = timestamppb.New(updatedAt)
		projects = append(projects, project)
	}

	return projects, nil
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

	sqlQuery, err := sqlx.GetProjectStatsQuery()
	if err != nil {
		return nil, fmt.Errorf("failed to build get project stats query: %w", err)
	}

	var stargazers sql.NullInt32
	var openIssues sql.NullInt32
	var updatedAt time.Time

	if err = ds.db.QueryRowContext(ctx, sqlQuery, sqlx.GetProjectStatsArgs(repo)...).Scan(&stargazers, &openIssues, &updatedAt); err != nil {
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
	stats.UpdatedAt = timestamppb.New(updatedAt)

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

	sqlQuery, err := sqlx.UpSertProjectStatsQuery()
	if err != nil {
		return fmt.Errorf("failed to build upsert project stats query: %w", err)
	}

	if _, err = ds.db.ExecContext(ctx, sqlQuery, sqlx.UpSertProjectStatsArgs(repo, stats)...); err != nil {
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
