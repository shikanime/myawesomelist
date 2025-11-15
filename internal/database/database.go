package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"myawesomelist.shikanime.studio/internal/agent"
	"myawesomelist.shikanime.studio/internal/config"
	dbpgx "myawesomelist.shikanime.studio/internal/database/pgx"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

type Database struct {
	pg  *pgxpool.Pool
	emb *agent.Embeddings
}

// NewForConfig constructs a Database using the provided config.
// It initializes the pgx pool and embeddings internally.
func NewForConfig(cfg *config.Config) (*Database, error) {
	pg, err := dbpgx.NewClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewClientWithPgxAndEmbedding(pg, agent.NewEmbeddingsForConfig(cfg)), nil
}

// NewForConfigWithEmbeddingsOptions constructs a Database using cfg and forwards embeddings options.
func NewForConfigWithEmbeddingsOptions(
	cfg *config.Config,
	opts ...agent.EmbeddingsOption,
) (*Database, error) {
	pg, err := dbpgx.NewClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewClientWithPgxAndEmbedding(pg, agent.NewEmbeddingsForConfig(cfg, opts...)), nil
}

// NewClientWithPgxAndEmbedding constructs a Database using the provided pgx pool and embeddings.
func NewClientWithPgxAndEmbedding(pg *pgxpool.Pool, emb *agent.Embeddings) *Database {
	return &Database{pg: pg, emb: emb}
}

// Ping verifies the provided database connection is available
func (db *Database) Ping(ctx context.Context) error {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.Ping")
	defer span.End()
	if db.pg == nil {
		return fmt.Errorf("database connection not available")
	}
	return db.pg.Ping(ctx)
}

func (db *Database) Close() error {
	if db.pg == nil {
		return nil
	}
	db.pg.Close()
	return nil
}

func (db *Database) UpsertRepositories(
	ctx context.Context,
	repos []*myawesomelistv1.Repository,
) ([]*Repository, error) {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.UpsertRepositories")
	span.SetAttributes(attribute.Int("repos_len", len(repos)))
	defer span.End()
	if db.pg == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	if len(repos) == 0 {
		return nil, nil
	}
	rows := make([]Repository, 0, len(repos))
	for _, pr := range repos {
		rows = append(rows, Repository{Hostname: pr.Hostname, Owner: pr.Owner, Repo: pr.Repo})
	}
	b := &pgx.Batch{}
	for i := range rows {
		b.Queue(
			UpsertRepositoryQuery,
			rows[i].Hostname,
			rows[i].Owner,
			rows[i].Repo,
		)
	}
	slog.DebugContext(ctx, "upsert repositories queued", "count", len(rows))
	br := db.pg.SendBatch(ctx, b)
	defer br.Close()
	for i := range rows {
		var id int64
		if err := br.QueryRow().Scan(&id); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("upsert repository failed: %w", err)
		}
		rows[i].ID = uint64(id)
	}
	out := make([]*Repository, 0, len(rows))
	for i := range rows {
		out = append(out, &rows[i])
	}
	slog.DebugContext(ctx, "upsert repositories done", "count", len(out))
	return out, nil
}

// ListCollections retrieves collections for the provided repos from the database
func (db *Database) ListCollections(
	ctx context.Context,
	repos []*myawesomelistv1.Repository,
) ([]*myawesomelistv1.Collection, error) {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.ListCollections")
	span.SetAttributes(attribute.Int("repos_len", len(repos)))
	defer span.End()
	if db.pg == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	query, args, err := RenderListCollectionsQuery(repos)
	if err != nil {
		return nil, err
	}
	slog.DebugContext(ctx, "list collections query", "sql", query, "args_len", len(args))
	cr, err := db.pg.Query(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("list collections query failed: %w", err)
	}
	defer cr.Close()
	var cols []Collection
	for cr.Next() {
		var c Collection
		var host, owner, repo string
		if err = cr.Scan(&c.ID, &c.RepositoryID, &c.Language, &c.UpdatedAt, &host, &owner, &repo); err != nil {
			return nil, err
		}
		slog.DebugContext(ctx, "list collections scanned", "collections", len(cols))
		c.Repository = Repository{ID: c.RepositoryID, Hostname: host, Owner: owner, Repo: repo}
		cols = append(cols, c)
	}
	// categories per collection
	if len(cols) == 0 {
		return nil, nil
	}
	ids := make([]uint64, len(cols))
	for i := range cols {
		ids[i] = cols[i].ID
	}
	catRows, err := db.pg.Query(ctx, CategoriesByCollectionIDsQuery, ids)
	if err == nil {
		defer catRows.Close()
		catsByCol := make(map[uint64][]Category)
		scannedCats, err := pgx.CollectRows(catRows, func(r pgx.CollectableRow) (Category, error) {
			var cat Category
			if err := r.Scan(&cat.ID, &cat.CollectionID, &cat.Name, &cat.UpdatedAt); err != nil {
				return Category{}, err
			}
			return cat, nil
		})
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		slog.DebugContext(ctx, "list collections scanned cats", "count", len(scannedCats))
		for i := range scannedCats {
			catsByCol[scannedCats[i].CollectionID] = append(
				catsByCol[scannedCats[i].CollectionID],
				scannedCats[i],
			)
		}
		for i := range cols {
			cols[i].Categories = catsByCol[cols[i].ID]
		}
	}
	// projects
	var catIDs []uint64
	for _, col := range cols {
		for _, cat := range col.Categories {
			catIDs = append(catIDs, cat.ID)
		}
	}
	if len(catIDs) > 0 {
		pr, err := db.pg.Query(ctx, ProjectsByCategoryIDsQuery, catIDs)
		if err == nil {
			defer pr.Close()
			pm := make(map[uint64][]Project)
			scannedProjs, err := pgx.CollectRows(pr, func(r pgx.CollectableRow) (Project, error) {
				var p Project
				var h, o, rp string
				if err := r.Scan(&p.ID, &p.CategoryID, &p.RepositoryID, &p.Name, &p.Description, &p.UpdatedAt, &h, &o, &rp); err != nil {
					return Project{}, err
				}
				p.Repository = Repository{ID: p.RepositoryID, Hostname: h, Owner: o, Repo: rp}
				return p, nil
			})
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return nil, err
			}
			slog.DebugContext(ctx, "list collections scanned projects", "count", len(scannedProjs))
			for i := range scannedProjs {
				pm[scannedProjs[i].CategoryID] = append(
					pm[scannedProjs[i].CategoryID],
					scannedProjs[i],
				)
			}
			for j := range cols {
				for i := range cols[j].Categories {
					cols[j].Categories[i].Projects = pm[cols[j].Categories[i].ID]
				}
			}
		}
	}
	var out []*myawesomelistv1.Collection
	for _, col := range cols {
		out = append(out, col.ToProto())
	}
	slog.DebugContext(ctx, "list collections done", "collections", len(out))
	return out, nil
}

// GetCollection retrieves a collection from the database
func (db *Database) GetCollection(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*myawesomelistv1.Collection, error) {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.GetCollection")
	span.SetAttributes(attribute.String("owner", repo.Owner), attribute.String("repo", repo.Repo))
	defer span.End()
	if db.pg == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	var rid uint64
	if err := db.pg.QueryRow(ctx, RepoIDQuery, repo.Hostname, repo.Owner, repo.Repo).Scan(&rid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to resolve repository: %w", err)
	}

	var col Collection
	var hostname, owner, repon string
	err := db.pg.QueryRow(ctx, CollectionByRepoIDQuery, rid).
		Scan(&col.ID, &col.RepositoryID, &col.Language, &col.UpdatedAt, &hostname, &owner, &repon)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}
	col.Repository = Repository{ID: col.RepositoryID, Hostname: hostname, Owner: owner, Repo: repon}
	slog.DebugContext(ctx, "get collection", "repo_id", rid, "categories", len(col.Categories))
	catRows, err := db.pg.Query(
		ctx,
		"SELECT id, collection_id, name, updated_at FROM categories WHERE collection_id=$1",
		col.ID,
	)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat Category
			if err := catRows.Scan(&cat.ID, &cat.CollectionID, &cat.Name, &cat.UpdatedAt); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return nil, err
			}
			col.Categories = append(col.Categories, cat)
		}
	}
	for i := range col.Categories {
		pr, err := db.pg.Query(
			ctx,
			"SELECT p.id, p.category_id, p.repository_id, p.name, p.description, p.updated_at, r.hostname, r.owner, r.repo FROM projects p JOIN repositories r ON r.id=p.repository_id WHERE p.category_id=$1",
			col.Categories[i].ID,
		)
		if err == nil {
			defer pr.Close()
			for pr.Next() {
				var p Project
				var h, o, rr string
				if err := pr.Scan(&p.ID, &p.CategoryID, &p.RepositoryID, &p.Name, &p.Description, &p.UpdatedAt, &h, &o, &rr); err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					return nil, err
				}
				p.Repository = Repository{ID: p.RepositoryID, Hostname: h, Owner: o, Repo: rr}
				col.Categories[i].Projects = append(col.Categories[i].Projects, p)
			}
		}
	}
	return col.ToProto(), nil
}

// UpsertCollections stores collections in the database
func (db *Database) UpsertCollections(
	ctx context.Context,
	cols []*myawesomelistv1.Collection,
) error {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.UpsertCollections")
	span.SetAttributes(attribute.Int("collections_len", len(cols)))
	defer span.End()
	if db.pg == nil {
		return fmt.Errorf("database connection not available")
	}
	repos := make([]*myawesomelistv1.Repository, 0, len(cols))
	for _, col := range cols {
		repos = append(repos, col.Repo)
	}
	rms, err := db.UpsertRepositories(ctx, repos)
	if err != nil {
		return err
	}
	slog.DebugContext(ctx, "upsert collections repos resolved", "count", len(rms))

	colms := make([]Collection, 0, len(cols))
	for i, col := range cols {
		colms = append(colms, Collection{RepositoryID: rms[i].ID, Language: col.Language})
	}
	b := &pgx.Batch{}
	for i := range colms {
		b.Queue(UpsertCollectionQuery, colms[i].RepositoryID, colms[i].Language)
	}
	slog.DebugContext(ctx, "upsert collections queued", "count", len(colms))
	br := db.pg.SendBatch(ctx, b)
	defer br.Close()
	for i := range colms {
		var id int64
		if err := br.QueryRow().Scan(&id); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("upsert collection failed: %w", err)
		}
		colms[i].ID = uint64(id)
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
		if err := db.UpsertCategories(ctx, cats); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("upsert categories failed: %w", err)
		}
		slog.DebugContext(ctx, "upsert categories done", "count", len(cats))
	}
	return nil
}

// SearchProjects executes a datastore-backed search across repositories.
func (db *Database) SearchProjects(
	ctx context.Context,
	q string,
	limit uint32,
	repos []*myawesomelistv1.Repository,
) ([]*myawesomelistv1.Project, error) {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.SearchProjects")
	span.SetAttributes(
		attribute.String("query", q),
		attribute.Int("repos_len", len(repos)),
		attribute.Int("limit", int(limit)),
	)
	defer span.End()
	if db.pg == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	var embedding *pgvector.Vector
	if q != "" {
		vecs, err := db.emb.EmbedProjects(
			ctx,
			[]*myawesomelistv1.Project{{Name: q, Description: ""}},
		)
		if err != nil {
			return nil, fmt.Errorf("generate query embedding failed: %w", err)
		}
		v := pgvector.NewVector(vecs[0])
		embedding = &v
	}
	slog.DebugContext(ctx, "search projects embedding", "used", embedding != nil)
	query, args, err := RenderSearchProjectsQuery(repos, embedding, int(limit))
	if err != nil {
		return nil, err
	}
	slog.DebugContext(
		ctx,
		"search projects query",
		"sql",
		query,
		"args_len",
		len(args),
		"limit",
		limit,
	)
	rows, err := db.pg.Query(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("search projects failed: %w", err)
	}
	defer rows.Close()
	var out []*myawesomelistv1.Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.UpdatedAt, &p.Repository.Hostname, &p.Repository.Owner, &p.Repository.Repo); err != nil {
			return nil, err
		}
		out = append(out, p.ToProto())
	}
	slog.DebugContext(ctx, "search projects results", "count", len(out))
	return out, rows.Err()
}

// Close closes the database connection

// GetProjectStats retrieves project stats from the datastore
func (db *Database) GetProjectStats(
	ctx context.Context,
	repo *myawesomelistv1.Repository,
) (*myawesomelistv1.ProjectStats, error) {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.GetProjectStats")
	span.SetAttributes(attribute.String("owner", repo.Owner), attribute.String("repo", repo.Repo))
	defer span.End()
	if db.pg == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	var rid uint64
	if err := db.pg.QueryRow(ctx, RepoIDQuery, repo.Hostname, repo.Owner, repo.Repo).Scan(&rid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to resolve repository: %w", err)
	}
	var ps ProjectStats
	err := db.pg.QueryRow(ctx, ProjectStatsByRepoIDQuery, rid).
		Scan(&ps.ID, &ps.RepositoryID, &ps.StargazersCount, &ps.OpenIssueCount, &ps.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("query project stats failed: %w", err)
	}
	return ps.ToProto(), nil
}

func (db *Database) GetProjectsStats(
	ctx context.Context,
	repos []*myawesomelistv1.Repository,
) ([]*myawesomelistv1.ProjectStats, error) {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.GetProjectsStats")
	span.SetAttributes(attribute.Int("repos_len", len(repos)))
	defer span.End()
	if db.pg == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	out := make([]*myawesomelistv1.ProjectStats, 0, len(repos))
	for _, repo := range repos {
		var rid uint64
		if err := db.pg.QueryRow(ctx, RepoIDQuery, repo.Hostname, repo.Owner, repo.Repo).Scan(&rid); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, fmt.Errorf("failed to resolve repository: %w", err)
		}
		var ps ProjectStats
		err := db.pg.QueryRow(ctx, ProjectStatsByRepoIDQuery, rid).
			Scan(&ps.ID, &ps.RepositoryID, &ps.StargazersCount, &ps.OpenIssueCount, &ps.UpdatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("query project stats failed: %w", err)
		}
		out = append(out, ps.ToProto())
	}
	return out, nil
}

// UpsertProjectStats stores project stats in the datastore
func (db *Database) UpsertProjectStats(
	ctx context.Context,
	repoID uint64,
	stats *myawesomelistv1.ProjectStats,
) error {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.UpsertProjectStats")
	span.SetAttributes(attribute.Int("repo_id", int(repoID)))
	defer span.End()
	slog.DebugContext(ctx, "upsert project stats", "repo_id", repoID)
	b := &pgx.Batch{}
	b.Queue(UpsertProjectStatsQuery, repoID, stats.StargazersCount, stats.OpenIssueCount)
	br := db.pg.SendBatch(ctx, b)
	defer br.Close()
	if _, err := br.Exec(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("upsert project stats failed: %w", err)
	}
	return nil
}

// UpsertCategories upserts categories and fills IDs in the provided slice
func (db *Database) UpsertCategories(
	ctx context.Context,
	categories []*Category,
) error {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.UpsertCategories")
	span.SetAttributes(attribute.Int("categories_len", len(categories)))
	defer span.End()
	if len(categories) > 0 {
		b := &pgx.Batch{}
		for i := range categories {
			b.Queue(UpsertCategoryQuery, categories[i].CollectionID, categories[i].Name)
		}
		br := db.pg.SendBatch(ctx, b)
		defer br.Close()
		for i := range categories {
			var id int64
			if err := br.QueryRow().Scan(&id); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return fmt.Errorf("upsert categories failed: %w", err)
			}
			categories[i].ID = uint64(id)
		}
	}
	var projects []*Project
	for _, cm := range categories {
		for _, project := range cm.Projects {
			rms, err := db.UpsertRepositories(
				ctx,
				[]*myawesomelistv1.Repository{project.Repository.ToProto()},
			)
			if err != nil || len(rms) == 0 {
				return fmt.Errorf("upsert project repository failed: %w", err)
			}
			project.RepositoryID = rms[0].ID
			project.CategoryID = cm.ID
			projects = append(projects, &project)
		}
	}
	if err := db.UpsertProjects(ctx, projects); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("upsert projects failed: %w", err)
	}
	return nil
}

// UpsertProjects upserts projects and their embeddings
func (db *Database) UpsertProjects(
	ctx context.Context,
	projects []*Project,
) error {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.UpsertProjects")
	span.SetAttributes(attribute.Int("projects_len", len(projects)))
	defer span.End()
	b := &pgx.Batch{}
	for _, project := range projects {
		b.Queue(
			UpsertProjectQuery,
			project.CategoryID,
			project.RepositoryID,
			project.Name,
			project.Description,
		)
	}
	br := db.pg.SendBatch(ctx, b)
	defer br.Close()
	for i := range projects {
		var id int64
		if err := br.QueryRow().Scan(&id); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("upsert project failed: %w", err)
		}
		projects[i].ID = uint64(id)
	}

	inputs := make([]*myawesomelistv1.Project, len(projects))
	for i, project := range projects {
		inputs[i] = project.ToProto()
	}
	vecs, err := db.emb.EmbedProjects(ctx, inputs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("generate project embeddings failed: %w", err)
	}
	pes := make([]ProjectEmbeddings, len(projects))
	for i, project := range projects {
		pes[i] = ProjectEmbeddings{
			ProjectID: project.ID,
			Embedding: pgvector.NewVector(vecs[i]),
		}
	}
	eb := &pgx.Batch{}
	for _, pe := range pes {
		eb.Queue(UpsertProjectEmbeddingQuery, pe.ProjectID, pe.Embedding)
	}
	ebr := db.pg.SendBatch(ctx, eb)
	defer ebr.Close()
	for range pes {
		if _, err := ebr.Exec(); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("upsert project embedding failed: %w", err)
		}
	}
	return nil
}

func (db *Database) UpsertProjectMetadata(
	ctx context.Context,
	repoID uint64,
	readme string,
) error {
	slog.DebugContext(ctx, "upsert project metadata", "repo_id", repoID, "readme_len", len(readme))
	b := &pgx.Batch{}
	b.Queue(UpsertProjectMetadataQuery, repoID, readme)
	br := db.pg.SendBatch(ctx, b)
	defer br.Close()
	if _, err := br.Exec(); err != nil {
		return fmt.Errorf("upsert project metadata failed: %w", err)
	}
	return nil
}
