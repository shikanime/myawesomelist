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

	q, err := sqlx.GetCollectionQuery(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to build joined collection query: %w", err)
	}

	rows, err := tx.QueryContext(ctx, q, sqlx.GetCollectionArgs(repo)...)
	if err != nil {
		return nil, fmt.Errorf("failed to query collection with join: %w", err)
	}
	defer rows.Close()

	col := &myawesomelistv1.Collection{}
	categoriesByID := make(map[int64]*myawesomelistv1.Category)
	var categoryOrder []int64

	for rows.Next() {
		var (
			categoryID        int64
			categoryName      sql.NullString
			categoryUpdatedAt time.Time

			projectName        sql.NullString
			projectDescription sql.NullString
			repoHostname       sql.NullString
			repoOwner          sql.NullString
			repoRepo           sql.NullString
			projectUpdatedAt   sql.NullTime

			collectionLanguage sql.NullString
			collectionUpdated  time.Time
		)

		if err := rows.Scan(
			&categoryID,
			&categoryName,
			&categoryUpdatedAt,
			&projectName,
			&projectDescription,
			&repoHostname,
			&repoOwner,
			&repoRepo,
			&projectUpdatedAt,
			&collectionLanguage,
			&collectionUpdated,
		); err != nil {
			return nil, fmt.Errorf("failed to scan joined collection row: %w", err)
		}

		// Assign collection fields once, after Scan
		if col.Language == "" && collectionLanguage.Valid {
			col.Language = collectionLanguage.String
			col.UpdatedAt = timestamppb.New(collectionUpdated)
		}

		// Get or create category struct
		category, exists := categoriesByID[categoryID]
		if !exists {
			category = &myawesomelistv1.Category{}
			if categoryName.Valid {
				category.Name = categoryName.String
			}
			category.UpdatedAt = timestamppb.New(categoryUpdatedAt)
			categoriesByID[categoryID] = category
			categoryOrder = append(categoryOrder, categoryID)
		}

		// Append project if present
		if projectName.Valid || repoHostname.Valid || repoOwner.Valid || repoRepo.Valid {
			p := &myawesomelistv1.Project{Repo: &myawesomelistv1.Repository{}}
			if projectName.Valid {
				p.Name = projectName.String
			}
			if projectDescription.Valid {
				p.Description = projectDescription.String
			}
			if repoHostname.Valid {
				p.Repo.Hostname = repoHostname.String
			}
			if repoOwner.Valid {
				p.Repo.Owner = repoOwner.String
			}
			if repoRepo.Valid {
				p.Repo.Repo = repoRepo.String
			}
			if projectUpdatedAt.Valid {
				p.UpdatedAt = timestamppb.New(projectUpdatedAt.Time)
			}
			category.Projects = append(category.Projects, p)
		}
	}

	for _, cid := range categoryOrder {
		col.Categories = append(col.Categories, categoriesByID[cid])
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit tx: %w", err)
	}

	slog.Debug("Retrieved collection (joined) from database",
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
		var (
			name        sql.NullString
			description sql.NullString
			hostname    sql.NullString
			owner       sql.NullString
			repo        sql.NullString
			updatedAt   time.Time
		)

		if err := rows.Scan(
			&name,
			&description,
			&hostname,
			&owner,
			&repo,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}

		project := &myawesomelistv1.Project{Repo: &myawesomelistv1.Repository{}}
		if name.Valid {
			project.Name = name.String
		}
		if description.Valid {
			project.Description = description.String
		}
		if hostname.Valid {
			project.Repo.Hostname = hostname.String
		}
		if owner.Valid {
			project.Repo.Owner = owner.String
		}
		if repo.Valid {
			project.Repo.Repo = repo.String
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

	stargazers := sql.NullInt32{}
	openIssues := sql.NullInt32{}
	updatedAt := time.Time{}

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
