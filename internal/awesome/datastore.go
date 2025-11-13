package awesome

import (
	"context"
	"errors"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"k8s.io/utils/ptr"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// DataStore wraps a SQL database and provides typed operations for cols.
type DataStore struct {
	db *gorm.DB
	ai *openai.Client
}

// NewDataStore constructs a DataStore using the provided sql.DB connection.
func NewDataStore(db *gorm.DB, ai *openai.Client) *DataStore {
	return &DataStore{db: db, ai: ai}
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
		Preload("Repository").
		Preload("Categories").
		Preload("Categories.Projects").
		Preload("Categories.Projects.Repository").
		Model(&Collection{}).
		Joins("JOIN repositories ON repositories.id = collections.repository_id")

	if len(repos) > 0 {
		db = db.Scopes(func(tx *gorm.DB) *gorm.DB {
			for i, r := range repos {
				cond := "(repositories.hostname = ? AND repositories.owner = ? AND repositories.repo = ?)"
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

	var r Repository
	if err := ds.db.WithContext(ctx).
		Where("hostname = ? AND owner = ? AND repo = ?", repo.Hostname, repo.Owner, repo.Repo).
		First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to resolve repository: %w", err)
	}

	var col Collection
	if err := ds.db.WithContext(ctx).
		Preload("Repository").
		Preload("Categories").
		Preload("Categories.Projects").
		Preload("Categories.Projects.Repository").
		Where("repository_id = ?", r.ID).
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
		// ensure repository exists
		var rm Repository
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
			DoNothing: true,
		}).Create(&Repository{
			Hostname: col.Repo.Hostname,
			Owner:    col.Repo.Owner,
			Repo:     col.Repo.Repo,
		}).Error; err != nil {
			return fmt.Errorf("upsert repository failed: %w", err)
		}
		if err := tx.Where("hostname = ? AND owner = ? AND repo = ?",
			col.Repo.Hostname, col.Repo.Owner, col.Repo.Repo).First(&rm).Error; err != nil {
			return fmt.Errorf("load repository failed: %w", err)
		}

		// upsert collection by repository_id
		colm := Collection{
			RepositoryID: rm.ID,
			Language:     col.Language,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "repository_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"language": colm.Language, "updated_at": gorm.Expr("NOW()")}),
		}).Create(&colm).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("upsert collection failed: %w", err)
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
				// ensure project repository exists
				var prm Repository
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
					DoNothing: true,
				}).Create(&Repository{
					Hostname: p.Repo.Hostname,
					Owner:    p.Repo.Owner,
					Repo:     p.Repo.Repo,
				}).Error; err != nil {
					return fmt.Errorf("upsert project repository failed: %w", err)
				}
				if err := tx.Where("hostname = ? AND owner = ? AND repo = ?",
					p.Repo.Hostname, p.Repo.Owner, p.Repo.Repo).First(&prm).Error; err != nil {
					return fmt.Errorf("load project repository failed: %w", err)
				}

				pm := Project{
					CategoryID:   cm.ID,
					RepositoryID: prm.ID,
					Name:         p.Name,
					Description:  p.Description,
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{
						{Name: "category_id"}, {Name: "repository_id"},
					},
					DoUpdates: clause.Assignments(map[string]interface{}{
						"name":        pm.Name,
						"description": pm.Description,
						"updated_at":  gorm.Expr("NOW()"),
					}),
				}).Create(&pm).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
					return fmt.Errorf("upsert project failed: %w", err)
				}

				embeddingRes, err := ds.ai.Embeddings.New(ctx, openai.EmbeddingNewParams{
					Input: openai.EmbeddingNewParamsInputUnion{
						OfString: openai.String(p.Name + " " + p.Description),
					},
					Model: openai.EmbeddingModel(GetEmbeddingModel()),
				})
				if err != nil {
					return fmt.Errorf("generate project embedding failed: %w", err)
				}

				embedding := make([]float32, len(embeddingRes.Data[0].Embedding))
				for i := range embedding {
					embedding[i] = float32(embeddingRes.Data[0].Embedding[i])
				}
				pe := ProjectEmbeddings{
					ProjectID: pm.ID,
					Embedding: pgvector.NewVector(embedding),
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "project_id"}},
					DoUpdates: clause.Assignments(map[string]interface{}{
						"embedding":  pe.Embedding,
						"updated_at": gorm.Expr("NOW()"),
					}),
				}).Create(&pe).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
					return fmt.Errorf("upsert project embedding failed: %w", err)
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
		Preload("Repository").
		Joins("JOIN categories ON categories.id = projects.category_id").
		Joins("JOIN collections ON collections.id = categories.collection_id").
		Joins("JOIN repositories cr ON cr.id = collections.repository_id")

	if len(repos) > 0 {
		db = db.Scopes(func(tx *gorm.DB) *gorm.DB {
			for i, r := range repos {
				cond := "(cr.hostname = ? AND cr.owner = ? AND cr.repo = ?)"
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
		db = db.Where(
			"(projects.name ILIKE ? OR projects.description ILIKE ?)",
			"%"+q+"%",
			"%"+q+"%",
		)
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

	var r Repository
	if err := ds.db.WithContext(ctx).
		Where("hostname = ? AND owner = ? AND repo = ?", repo.Hostname, repo.Owner, repo.Repo).
		First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to resolve repository: %w", err)
	}

	var ps ProjectStats
	err := ds.db.WithContext(ctx).
		Where("repository_id = ?", r.ID).
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
	var r Repository
	if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
		DoNothing: true,
	}).Create(&Repository{
		Hostname: repo.Hostname,
		Owner:    repo.Owner,
		Repo:     repo.Repo,
	}).Error; err != nil {
		return fmt.Errorf("upsert repository failed: %w", err)
	}
	if err := ds.db.WithContext(ctx).
		Where("hostname = ? AND owner = ? AND repo = ?", repo.Hostname, repo.Owner, repo.Repo).
		First(&r).Error; err != nil {
		return fmt.Errorf("load repository failed: %w", err)
	}

	ps := ProjectStats{
		RepositoryID:    r.ID,
		StargazersCount: ptr.To(stats.GetStargazersCount()),
		OpenIssueCount:  ptr.To(stats.GetOpenIssueCount()),
	}

	return ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "repository_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"stargazers_count": ps.StargazersCount,
			"open_issue_count": ps.OpenIssueCount,
			"updated_at":       gorm.Expr("NOW()"),
		}),
	}).Create(&ps).Error
}

func (ds *DataStore) UpSertProjectMetadata(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
	readme []byte,
) error {
	var r Repository
	if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
		DoNothing: true,
	}).Create(&Repository{
		Hostname: repo.Hostname,
		Owner:    repo.Owner,
		Repo:     repo.Repo,
	}).Error; err != nil {
		return fmt.Errorf("upsert repository failed: %w", err)
	}
	if err := ds.db.WithContext(ctx).
		Where("hostname = ? AND owner = ? AND repo = ?", repo.Hostname, repo.Owner, repo.Repo).
		First(&r).Error; err != nil {
		return fmt.Errorf("load repository failed: %w", err)
	}

	pm := ProjectMetadata{
		RepositoryID: r.ID,
		Readme:       string(readme),
	}

	return ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "repository_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"readme":     pm.Readme,
			"updated_at": gorm.Expr("NOW()"),
		}),
	}).Create(&pm).Error
}
