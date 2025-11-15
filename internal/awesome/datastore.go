package awesome

import (
	"context"
	"errors"
	"fmt"

	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// DataStore wraps a SQL database and provides typed operations for cols.
type DataStore struct {
	db  *gorm.DB
	emb *Embeddings
}

// NewDataStore constructs a DataStore using the provided sql.DB connection.
func NewDataStore(db *gorm.DB, emb *Embeddings) *DataStore {
	return &DataStore{db: db, emb: emb}
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
	if len(repos) == 0 {
		return nil, nil
	}
	rows := make([]Repository, 0, len(repos))
	for _, pr := range repos {
		rows = append(rows, Repository{Hostname: pr.Hostname, Owner: pr.Owner, Repo: pr.Repo})
	}
	if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hostname"}, {Name: "owner"}, {Name: "repo"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"updated_at": gorm.Expr("NOW()")}),
	}).Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).Create(&rows).Error; err != nil {
		return nil, fmt.Errorf("upsert repository failed: %w", err)
	}
	out := make([]*Repository, 0, len(rows))
	for i := range rows {
		out = append(out, &rows[i])
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
	if len(cols) == 0 {
		return nil
	}
	return ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repos := make([]*myawesomelistv1.Repository, 0, len(cols))
		for _, col := range cols {
			repos = append(repos, col.Repo)
		}
		rms, err := NewDataStore(tx, ds.emb).UpsertRepositories(ctx, repos)
		if err != nil {
			return err
		}

		colms := make([]Collection, 0, len(cols))
		for i, col := range cols {
			colms = append(colms, Collection{RepositoryID: rms[i].ID, Language: col.Language})
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "repository_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"language": gorm.Expr("EXCLUDED.language"), "updated_at": gorm.Expr("NOW()")}),
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
			if err := NewDataStore(tx, ds.emb).UpsertCategories(ctx, cats); err != nil {
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
		queryVec, err := ds.emb.EmbedProject(ctx, q, "")
		if err != nil {
			return nil, fmt.Errorf("generate query embedding failed: %w", err)
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
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}
	if len(stats) == 0 {
		return nil
	}
	if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "repository_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"stargazers_count": gorm.Expr("EXCLUDED.stargazers_count"),
			"open_issue_count": gorm.Expr("EXCLUDED.open_issue_count"),
			"updated_at":       gorm.Expr("NOW()"),
		}),
	}).Create(&stats).Error; err != nil {
		return fmt.Errorf("upsert project stats failed: %w", err)
	}
	return nil
}

// UpsertCategories upserts categories and fills IDs in the provided slice
func (ds *DataStore) UpsertCategories(
	ctx context.Context,
	categories []*Category,
) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}
	if len(categories) == 0 {
		return nil
	}
	if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "collection_id"}, {Name: "name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"updated_at": gorm.Expr("NOW()")}),
	}).Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).Create(&categories).Error; err != nil {
		return fmt.Errorf("upsert categories failed: %w", err)
	}
	repoKey := func(r Repository) string { return r.Hostname + "/" + r.Owner + "/" + r.Repo }
	repoMap := make(map[string]*myawesomelistv1.Repository)
	for _, cm := range categories {
		for j := range cm.Projects {
			rp := cm.Projects[j].Repository.ToProto()
			repoMap[repoKey(cm.Projects[j].Repository)] = rp
		}
	}
	var repoList []*myawesomelistv1.Repository
	for _, rp := range repoMap {
		repoList = append(repoList, rp)
	}
	rms, err := ds.UpsertRepositories(ctx, repoList)
	if err != nil {
		return fmt.Errorf("upsert project repositories failed: %w", err)
	}
	idMap := make(map[string]uint64, len(rms))
	for _, r := range rms {
		idMap[repoKey(*r)] = r.ID
	}
	var projects []*Project
	for _, cm := range categories {
		for j := range cm.Projects {
			cm.Projects[j].RepositoryID = idMap[repoKey(cm.Projects[j].Repository)]
			cm.Projects[j].CategoryID = cm.ID
			projects = append(projects, &cm.Projects[j])
		}
	}
	if err := NewDataStore(ds.db, ds.emb).UpsertProjects(ctx, projects); err != nil {
		return fmt.Errorf("upsert projects failed: %w", err)
	}
	return nil
}

// UpsertProjects upserts projects and their embeddings
func (ds *DataStore) UpsertProjects(
	ctx context.Context,
	projects []*Project,
) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}
	if len(projects) == 0 {
		return nil
	}
	if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "category_id"}, {Name: "repository_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"name":        gorm.Expr("EXCLUDED.name"),
			"description": gorm.Expr("EXCLUDED.description"),
			"updated_at":  gorm.Expr("NOW()"),
		}),
	}).Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).Create(&projects).Error; err != nil {
		return fmt.Errorf("upsert project failed: %w", err)
	}

	inputs := make([]ProjectInput, len(projects))
	for i := range projects {
		inputs[i] = ProjectInput{Name: projects[i].Name, Description: projects[i].Description}
	}
	vecs, err := ds.emb.EmbedProjects(ctx, inputs)
	if err != nil {
		return fmt.Errorf("generate project embeddings failed: %w", err)
	}
	pes := make([]ProjectEmbeddings, len(projects))
	for i := range projects {
		pes[i] = ProjectEmbeddings{ProjectID: projects[i].ID, Embedding: pgvector.NewVector(vecs[i])}
	}
	if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "project_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"embedding":  gorm.Expr("EXCLUDED.embedding"),
			"updated_at": gorm.Expr("NOW()"),
		}),
	}).Create(&pes).Error; err != nil {
		return fmt.Errorf("upsert project embedding failed: %w", err)
	}
	return nil
}

func (ds *DataStore) UpsertProjectMetadata(
	ctx context.Context,
	metas []*ProjectMetadata,
) error {
	if ds.db == nil {
		return fmt.Errorf("database connection not available")
	}
	if len(metas) == 0 {
		return nil
	}
	if err := ds.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "repository_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"readme":     gorm.Expr("EXCLUDED.readme"),
			"updated_at": gorm.Expr("NOW()"),
		}),
	}).Create(&metas).Error; err != nil {
		return fmt.Errorf("upsert project metadata failed: %w", err)
	}
	return nil
}
