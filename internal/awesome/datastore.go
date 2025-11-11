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

	if len(repos) == 0 {
		return nil, nil
	}

	rows, err := gorm.G[Collection](ds.db).
		Joins(
			clause.JoinTarget{
				Type:        clause.LeftJoin,
				Association: "Categories",
			},
			nil,
		).
		Joins(
			clause.JoinTarget{
				Type:        clause.LeftJoin,
				Association: "Categories.Projects",
			},
			nil,
		).
		Where(func(tx *gorm.DB) *gorm.DB {
			for i, r := range repos {
				cond := "(collections.hostname = ? AND collections.owner = ? AND collections.repo = ?)"
				if i == 0 {
					tx = tx.Where(cond, r.Hostname, r.Owner, r.Repo)
				} else {
					tx = tx.Or(cond, r.Hostname, r.Owner, r.Repo)
				}
			}
			return tx
		}).
		Find(ctx)
	if err != nil {
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

	col, err := gorm.G[Collection](ds.db).
		Joins(
			clause.JoinTarget{
				Type:        clause.LeftJoin,
				Association: "Categories",
			},
			nil,
		).
		Joins(
			clause.JoinTarget{
				Type:        clause.LeftJoin,
				Association: "Categories.Projects",
			},
			nil,
		).
		Where("collections.hostname = ? AND collections.owner = ? AND collections.repo = ?", repo.Hostname, repo.Owner, repo.Repo).
		First(ctx)
	if err != nil {
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
			Hostname: col.Repo.Hostname,
			Owner:    col.Repo.Owner,
			Repo:     col.Repo.Repo,
			Language: col.Language,
		}
		if err := gorm.G[Collection](ds.db, clause.OnConflict{
			Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"language": colm.Language, "updated_at": gorm.Expr("NOW()")}),
		}).
			Create(ctx, &colm); err != nil {
			if !errors.Is(err, gorm.ErrDuplicatedKey) {
				return fmt.Errorf("upsert collection failed: %w", err)
			}
		}

		for _, cat := range col.Categories {
			cm := Category{
				CollectionID: colm.ID,
				Name:         cat.Name,
			}
			if err := gorm.G[Category](ds.db, clause.OnConflict{
				Columns:   []clause.Column{{Name: "collection_id"}, {Name: "name"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"updated_at": gorm.Expr("NOW()")}),
			}).
				Create(ctx, &cm); err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
				return fmt.Errorf("upsert category failed: %w", err)
			}

			for _, p := range cat.Projects {
				pm := Project{
					CategoryID:  cm.ID,
					Name:        p.Name,
					Description: p.Description,
					Hostname:    p.Repo.Hostname,
					Owner:       p.Repo.Owner,
					Repo:        p.Repo.Repo,
				}
				if err := gorm.G[Project](ds.db, clause.OnConflict{
					Columns: []clause.Column{
						{Name: "category_id"}, {Name: "hostname"}, {Name: "owner"}, {Name: "repo"},
					},
					DoUpdates: clause.Assignments(map[string]interface{}{
						"name":        pm.Name,
						"description": pm.Description,
						"updated_at":  gorm.Expr("NOW()"),
					}),
				}).
					Create(ctx, &pm); err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
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

	qq := gorm.G[Project](ds.db).
		Joins(
			clause.JoinTarget{
				Type:        clause.LeftJoin,
				Association: "Categories",
			},
			nil,
		).
		Joins(
			clause.JoinTarget{
				Type:        clause.LeftJoin,
				Association: "Categories.Projects",
			},
			nil,
		).
		Where(func(tx *gorm.DB) *gorm.DB {
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

	if q != "" {
		qq = qq.Where("(projects.name ILIKE ? OR projects.description ILIKE ?)", "%"+q+"%", "%"+q+"%")
	}

	rows, err := qq.Limit(int(limit)).
		Order("projects.updated_at DESC").
		Find(ctx)
	if err != nil {
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

	ps, err := gorm.G[ProjectStats](ds.db).
		Where("hostname = ? AND owner = ? AND repo = ?", repo.Hostname, repo.Owner, repo.Repo).
		First(ctx)
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

	return gorm.G[ProjectStats](ds.db, clause.OnConflict{
		Columns: []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"stargazers_count": ps.StargazersCount,
			"open_issue_count": ps.OpenIssueCount,
			"updated_at":       gorm.Expr("NOW()"),
		}),
	}).
		Create(ctx, &ps)
}
