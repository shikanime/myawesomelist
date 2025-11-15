package awesome

import (
	"context"
	"errors"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (ds *DataStore) UpsertRepositories(
	ctx context.Context,
	repos []*myawesomelistv1.Repository,
) ([]*Repository, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	out := make([]*Repository, 0, len(repos))
	for _, pr := range repos {
		r := &Repository{Hostname: pr.Hostname, Owner: pr.Owner, Repo: pr.Repo}
		if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"updated_at": gorm.Expr("NOW()")}),
		}).Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).Create(r).Error; err != nil {
			return nil, fmt.Errorf("upsert repository failed: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// ListCollections retrieves collections for the provided repos from the database
func (ds *DataStore) ListCollections(
	ctx context.Context,
	repos []*myawesomelistv1.Repository,
) ([]*myawesomelistv1.Collection, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

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

// UpsertCollections stores collections in the database
func (ds *DataStore) UpsertCollections(
	ctx context.Context,
	cols []*myawesomelistv1.Collection,
) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}
	return ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repos := make([]*myawesomelistv1.Repository, 0, len(cols))
		for _, col := range cols {
			repos = append(repos, col.Repo)
		}
		rms, err := NewDataStore(tx, ds.ai).UpsertRepositories(ctx, repos)
		if err != nil {
			return err
		}

		colms := make([]Collection, 0, len(cols))
		for i, col := range cols {
			colms = append(colms, Collection{RepositoryID: rms[i].ID, Language: col.Language})
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "repository_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"language": gorm.Expr("excluded.language"), "updated_at": gorm.Expr("NOW()")}),
		}).Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).Create(&colms).Error; err != nil {
			return fmt.Errorf("upsert collection failed: %w", err)
		}

		for i, col := range cols {
			cats := make([]*Category, 0, len(col.Categories))
			for _, cat := range col.Categories {
				c := &Category{CollectionID: colms[i].ID, Name: cat.Name}
				for _, p := range cat.Projects {
					c.Projects = append(
						c.Projects,
						Project{
							Repository:  RepositoryFromProto(p.Repo),
							Name:        p.Name,
							Description: p.Description,
						},
					)
				}
				cats = append(cats, c)
			}
			if err := NewDataStore(tx, ds.ai).UpsertCategories(ctx, cats); err != nil {
				return fmt.Errorf("upsert categories failed: %w", err)
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
		embeddingRes, err := ds.ai.Embeddings.New(ctx, openai.EmbeddingNewParams{
			Input: openai.EmbeddingNewParamsInputUnion{
				OfString: openai.String(q),
			},
			Model: openai.EmbeddingModel(GetEmbeddingModel()),
		})
		if err != nil {
			return nil, fmt.Errorf("generate query embedding failed: %w", err)
		}
		queryVec := make([]float32, len(embeddingRes.Data[0].Embedding))
		for i := range queryVec {
			queryVec[i] = float32(embeddingRes.Data[0].Embedding[i])
		}
		db = db.Joins("JOIN project_embeddings pe ON pe.project_id = projects.id").
			Order(clause.Expr{SQL: "pe.embedding <-> ?", Vars: []interface{}{pgvector.NewVector(queryVec)}})
	}

	var rows []Project
	if err := db.Limit(int(limit)).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("search projects failed: %w", err)
	}

	var out []*myawesomelistv1.Project
	for _, r := range rows {
		out = append(out, r.ToProto())
	}
	return out, nil
}

// Close closes the database connection

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

func (ds *DataStore) GetProjectsStats(
	ctx context.Context,
	repos []*myawesomelistv1.Repository,
) ([]*myawesomelistv1.ProjectStats, error) {
	if ds.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	out := make([]*myawesomelistv1.ProjectStats, 0, len(repos))
	for _, repo := range repos {
		var r Repository
		if err := ds.db.WithContext(ctx).
			Where("hostname = ? AND owner = ? AND repo = ?", repo.Hostname, repo.Owner, repo.Repo).
			First(&r).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, fmt.Errorf("failed to resolve repository: %w", err)
		}
		var ps ProjectStats
		err := ds.db.WithContext(ctx).
			Where("repository_id = ?", r.ID).
			First(&ps).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, fmt.Errorf("query project stats failed: %w", err)
		}
		out = append(out, ps.ToProto())
	}
	return out, nil
}

// UpsertProjectStats stores project stats in the datastore
func (ds *DataStore) UpsertProjectStats(
	ctx context.Context,
	stats []*ProjectStats,
) error {
	if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "repository_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"stargazers_count": gorm.Expr("EXCLUDED.stargazers_count"),
			"open_issue_count": gorm.Expr("EXCLUDED.open_issue_count"),
			"updated_at":       gorm.Expr("NOW()"),
		}),
	}).Create(stats).Error; err != nil {
		return fmt.Errorf("upsert project stats failed: %w", err)
	}
	return nil
}

// UpsertCategories upserts categories and fills IDs in the provided slice
func (ds *DataStore) UpsertCategories(
	ctx context.Context,
	categories []*Category,
) error {
	if len(categories) > 0 {
		if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "collection_id"}, {Name: "name"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"updated_at": gorm.Expr("NOW()")}),
		}).Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).Create(categories).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("upsert categories failed: %w", err)
		}
	}
	var projects []*Project
	for _, cm := range categories {
		for j := range cm.Projects {
			rms, err := ds.UpsertRepositories(
				ctx,
				[]*myawesomelistv1.Repository{cm.Projects[j].Repository.ToProto()},
			)
			if err != nil {
				return fmt.Errorf("upsert project repository failed: %w", err)
			}
			cm.Projects[j].RepositoryID = rms[0].ID
			cm.Projects[j].CategoryID = cm.ID
			projects = append(projects, &cm.Projects[j])
		}
	}
	if err := NewDataStore(ds.db, ds.ai).UpsertProjects(ctx, projects); err != nil {
		return fmt.Errorf("upsert projects failed: %w", err)
	}
	return nil
}

// UpsertProjects upserts projects and their embeddings
func (ds *DataStore) UpsertProjects(
	ctx context.Context,
	projects []*Project,
) error {
	for i := range projects {
		pm := projects[i]
		if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "category_id"}, {Name: "repository_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"name":        pm.Name,
				"description": pm.Description,
				"updated_at":  gorm.Expr("NOW()"),
			}),
		}).Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).Create(pm).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("upsert project failed: %w", err)
		}
		embeddingRes, err := ds.ai.Embeddings.New(ctx, openai.EmbeddingNewParams{
			Input: openai.EmbeddingNewParamsInputUnion{
				OfString: openai.String(pm.Name + " " + pm.Description),
			},
			Model: openai.EmbeddingModel(GetEmbeddingModel()),
		})
		if err != nil {
			return fmt.Errorf("generate project embedding failed: %w", err)
		}
		embedding := make([]float32, len(embeddingRes.Data[0].Embedding))
		for j := range embedding {
			embedding[j] = float32(embeddingRes.Data[0].Embedding[j])
		}
		pe := ProjectEmbeddings{
			ProjectID: pm.ID,
			Embedding: pgvector.NewVector(embedding),
		}
		if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "project_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"embedding":  pe.Embedding,
				"updated_at": gorm.Expr("NOW()"),
			}),
		}).Create(&pe).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("upsert project embedding failed: %w", err)
		}
	}
	return nil
}

func (ds *DataStore) UpsertProjectMetadata(
	ctx context.Context,
	metas []*ProjectMetadata,
) error {
	for _, pm := range metas {
		if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "repository_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"readme":     pm.Readme,
				"updated_at": gorm.Expr("NOW()"),
			}),
		}).Create(pm).Error; err != nil {
			return fmt.Errorf("upsert project metadata failed: %w", err)
		}
	}
	return nil
}
