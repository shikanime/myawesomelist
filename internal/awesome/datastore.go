package awesome

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"k8s.io/utils/ptr"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// DataStore wraps a SQL database and provides typed operations for cols.
type DataStore struct {
	db *gorm.DB
}

// NewDataStore constructs a DataStore using the provided sql.DB connection.
func NewDataStore(db *gorm.DB) *DataStore {
	return &DataStore{db: db}
}

// Ping verifies the provided database connection is available
func (ds *DataStore) Ping(ctx context.Context) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}
	db, err := ds.db.DB()
	if err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return db.PingContext(ctx)
}

// ListCollections retrieves collections for the provided repos from the database
func (ds *DataStore) ListCollections(
	ctx context.Context,
	repos []*myawesomelistv1.Repository,
) ([]*myawesomelistv1.Collection, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	// Build OR predicates for target repos
	db := ds.db.WithContext(ctx).
		Preload("Categories").
		Preload("Categories.Projects").
		Model(&Collection{})

	if len(repos) == 0 {
		db = db.Scopes(func(tx *gorm.DB) *gorm.DB {
			for i, r := range repos {
				cond := "(hostname = ? AND owner = ? AND repo = ?)"
				if i == 0 {
					tx = tx.Where(cond, r.Hostname, r.Owner, r.Repo)
				} else {
					tx = tx.Or(cond, r.Hostname, r.Owner, r.Repo)
				}
			}
			return tx
		})
	}

	var rows []Collection
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list collections query failed: %w", err)
	}

	var out []*myawesomelistv1.Collection
	for _, r := range rows {
		out = append(out, r.ToProto())
	}
	return out, nil
}

// GetCollection retrieves a collection from the database
func (ds *DataStore) GetCollection(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*myawesomelistv1.Collection, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	var col Collection
	if err := ds.db.WithContext(ctx).
		Preload("Categories").
		Preload("Categories.Projects").
		Where("hostname = ? AND owner = ? AND repo = ?", repo.Hostname, repo.Owner, repo.Repo).
		First(&col).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}

	return col.ToProto(), nil
}

// UpSertCollection stores a collection in the database
func (ds *DataStore) UpSertCollection(
	ctx context.Context,
	col *myawesomelistv1.Collection,
) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}

	return ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		colm := Collection{
			Repo: Repository{
				Hostname: col.Repo.Hostname,
				Owner:    col.Repo.Owner,
				Repo:     col.Repo.Repo,
			},
			Language: col.Language,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"language": colm.Language, "updated_at": gorm.Expr("NOW()")}),
		}).Create(&colm).Error; err != nil {
			// If exists, load its ID
			if !errors.Is(err, gorm.ErrDuplicatedKey) {
				return fmt.Errorf("upsert collection failed: %w", err)
			}
		}

		// Upsert categories and projects
		for _, cat := range col.Categories {
			cm := Category{
				CollectionID: colm.ID,
				Name:         cat.Name,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "collection_id"}, {Name: "name"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"updated_at": gorm.Expr("NOW()")}),
			}).Create(&cm).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
				return fmt.Errorf("upsert category failed: %w", err)
			}

			for _, p := range cat.Projects {
				pm := Project{
					CategoryID:   cm.ID,
					Name:         p.Name,
					Description:  p.Description,
					RepoHostname: p.Repo.Hostname,
					RepoOwner:    p.Repo.Owner,
					RepoRepo:     p.Repo.Repo,
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{
						{Name: "category_id"}, {Name: "repo_hostname"}, {Name: "repo_owner"}, {Name: "repo_repo"},
					},
					DoUpdates: clause.Assignments(map[string]interface{}{
						"name":        pm.Name,
						"description": pm.Description,
						"updated_at":  gorm.Expr("NOW()"),
					}),
				}).Create(&pm).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
					return fmt.Errorf("upsert project failed: %w", err)
				}
			}
		}
		return nil
	})
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

	db := ds.db.WithContext(ctx).Model(&Project{}).
		Joins("JOIN categories ON categories.id = projects.category_id").
		Joins("JOIN collections ON collections.id = categories.collection_id")

	// Filter by repos if provided
	if len(repos) > 0 {
		// Group OR predicates for allowed repos with Scopes to avoid driver binding issues
		db = db.Scopes(func(tx *gorm.DB) *gorm.DB {
			for i, r := range repos {
				cond := "(collections.hostname = ? AND collections.owner = ? AND collections.repo = ?)"
				if i == 0 {
					tx = tx.Where(cond, r.Hostname, r.Owner, r.Repo)
				} else {
					tx = tx.Or(cond, r.Hostname, r.Owner, r.Repo)
				}
			}
			return tx
		})
	}

	// Basic text search on name/description
	if q != "" {
		db = db.Where("(projects.name ILIKE ? OR projects.description ILIKE ?)", "%"+q+"%", "%"+q+"%")
	}

	var rows []Project
	if err := db.Limit(int(limit)).Order("projects.updated_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("search projects failed: %w", err)
	}

	var out []*myawesomelistv1.Project
	for _, r := range rows {
		out = append(out, r.ToProto())
	}
	return out, nil
}

// Close closes the database connection
func (ds *DataStore) Close() error {
	if ds.db == nil {
		return nil
	}
	sqlDB, err := ds.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetProjectStats retrieves project stats from the datastore
func (ds *DataStore) GetProjectStats(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*myawesomelistv1.ProjectStats, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	var ps ProjectStats
	err := ds.db.WithContext(ctx).
		Where("hostname = ? AND owner = ? AND repo = ?", repo.Hostname, repo.Owner, repo.Repo).
		First(&ps).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("query project stats failed: %w", err)
	}

	return ps.ToProto(), nil
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

	ps := ProjectStats{
		Hostname:        repo.Hostname,
		Owner:           repo.Owner,
		Repo:            repo.Repo,
		StargazersCount: ptr.To(stats.GetStargazersCount()),
		OpenIssueCount:  ptr.To(stats.GetOpenIssueCount()),
	}

	return ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"stargazers_count": ps.StargazersCount,
			"open_issue_count": ps.OpenIssueCount,
			"updated_at":       gorm.Expr("NOW()"),
		}),
	}).Create(&ps).Error
}
