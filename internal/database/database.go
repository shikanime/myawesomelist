package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
	"myawesomelist.shikanime.studio/internal/config"
	dbpgx "myawesomelist.shikanime.studio/internal/database/pgx"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

type Repository struct {
	ID       uint64
	Hostname string
	Owner    string
	Repo     string
}

type Project struct {
	ID           uint64
	CategoryID   uint64
	RepositoryID uint64
	Repository   Repository
	Name         string
	Description  string
	UpdatedAt    time.Time
}

type Category struct {
	ID           uint64
	CollectionID uint64
	Name         string
	Projects     []Project
	UpdatedAt    time.Time
}

type Collection struct {
	ID           uint64
	RepositoryID uint64
	Repository   Repository
	Language     string
	Categories   []Category
	UpdatedAt    time.Time
}

type ProjectStats struct {
	ID              uint64
	RepositoryID    uint64
	StargazersCount *uint32
	OpenIssueCount  *uint32
	UpdatedAt       time.Time
}

type Database struct {
	pg *pgxpool.Pool
}

// NewForConfig constructs a Database using the provided config.
// It initializes the pgx pool and embeddings internally.
func NewForConfig(cfg *config.Config) (*Database, error) {
	pg, err := dbpgx.NewClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewClient(pg), nil
}

// NewClient constructs a Database using the provided pgx pool.
func NewClient(pg *pgxpool.Pool) *Database { return &Database{pg: pg} }

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

// UpsertRepositories upserts repositories into the database
func (db *Database) UpsertRepositories(
	ctx context.Context,
	repos []*UpsertRepositoryArgs,
) ([]*UpsertRepositoriesResult, error) {
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
	// queue upserts for each repository arg
	b := &pgx.Batch{}
	for i := range repos {
		b.Queue(UpsertRepositoryQuery, repos[i].Hostname, repos[i].Owner, repos[i].Repo)
	}
	slog.DebugContext(ctx, "upsert repositories queued", "count", len(repos))
	br := db.pg.SendBatch(ctx, b)
	defer br.Close()
	out := make([]*UpsertRepositoriesResult, 0, len(repos))
	for i := range repos {
		var id int64
		if err := br.QueryRow().Scan(&id); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("upsert repository failed: %w", err)
		}
		out = append(
			out,
			&UpsertRepositoriesResult{
				ID:       uint64(id),
				Hostname: repos[i].Hostname,
				Owner:    repos[i].Owner,
				Repo:     repos[i].Repo,
			},
		)
	}
	slog.DebugContext(ctx, "upsert repositories done", "count", len(out))
	return out, nil
}

// ListCollections retrieves collections for the provided repos from the database
func (db *Database) ListCollections(
	ctx context.Context,
	args ListCollectionsArgs,
) ([]*myawesomelistv1.Collection, error) {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.ListCollections")
	span.SetAttributes(attribute.Int("repos_len", len(args.Repos)))
	defer span.End()
	if db.pg == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	query, qargs, err := RenderListCollectionsQuery(args.Repos)
	if err != nil {
		return nil, err
	}
	slog.DebugContext(ctx, "list collections query", "sql", query, "args_len", len(qargs))
	cr, err := db.pg.Query(ctx, query, qargs...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("list collections query failed: %w", err)
	}
	defer cr.Close()
	type colRow struct {
		ID           uint64
		RepositoryID uint64
		Language     string
		UpdatedAt    time.Time
		Hostname     string
		Owner        string
		Repo         string
	}
	var cols []colRow
	for cr.Next() {
		var c colRow
		if err = cr.Scan(&c.ID, &c.RepositoryID, &c.Language, &c.UpdatedAt, &c.Hostname, &c.Owner, &c.Repo); err != nil {
			return nil, err
		}
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
	// local row structs for mapping
	type categoryRow struct {
		ID           uint64
		CollectionID uint64
		Name         string
		UpdatedAt    time.Time
	}
	type projectRow struct {
		ID           uint64
		CategoryID   uint64
		RepositoryID uint64
		Name         string
		Description  string
		UpdatedAt    time.Time
		Hostname     string
		Owner        string
		Repo         string
	}
	// predeclare maps to assemble output later
	catsByCol := make(map[uint64][]categoryRow)
	pm := make(map[uint64][]projectRow)
	catRows, err := db.pg.Query(ctx, CategoriesByCollectionIDsQuery, ids)
	if err == nil {
		defer catRows.Close()
		scannedCats, err := pgx.CollectRows(catRows, pgx.RowToStructByPos[categoryRow])
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
	}
	// projects
	var catIDs []uint64
	for _, col := range cols {
		for _, cat := range catsByCol[col.ID] {
			catIDs = append(catIDs, cat.ID)
		}
	}
	if len(catIDs) > 0 {
		pr, err := db.pg.Query(ctx, ProjectsByCategoryIDsQuery, catIDs)
		if err == nil {
			defer pr.Close()
			scannedProjs, err := pgx.CollectRows(pr, pgx.RowToStructByPos[projectRow])
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
			// accumulated in pm map
		}
	}
	var out []*myawesomelistv1.Collection
	for _, col := range cols {
		pc := &myawesomelistv1.Collection{
			Id:        col.ID,
			Language:  col.Language,
			UpdatedAt: timestamppb.New(col.UpdatedAt),
			Repo: &myawesomelistv1.Repository{
				Hostname: col.Hostname,
				Owner:    col.Owner,
				Repo:     col.Repo,
			},
		}
		for _, cat := range catsByCol[col.ID] {
			var ps []*myawesomelistv1.Project
			for _, p := range pm[cat.ID] {
				ps = append(
					ps,
					&myawesomelistv1.Project{
						Id:          p.ID,
						Name:        p.Name,
						Description: p.Description,
						Repo: &myawesomelistv1.Repository{
							Hostname: p.Hostname,
							Owner:    p.Owner,
							Repo:     p.Repo,
						},
						UpdatedAt: timestamppb.New(p.UpdatedAt),
					},
				)
			}
			pc.Categories = append(
				pc.Categories,
				&myawesomelistv1.Category{
					Id:        cat.ID,
					Name:      cat.Name,
					UpdatedAt: timestamppb.New(cat.UpdatedAt),
					Projects:  ps,
				},
			)
		}
		out = append(out, pc)
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
	pc := &myawesomelistv1.Collection{
		Id:        col.ID,
		Language:  col.Language,
		UpdatedAt: timestamppb.New(col.UpdatedAt),
		Repo: &myawesomelistv1.Repository{
			Hostname: col.Repository.Hostname,
			Owner:    col.Repository.Owner,
			Repo:     col.Repository.Repo,
		},
	}
	for _, cat := range col.Categories {
		pc.Categories = append(pc.Categories, &myawesomelistv1.Category{
			Id:        cat.ID,
			Name:      cat.Name,
			UpdatedAt: timestamppb.New(cat.UpdatedAt),
			Projects: func() []*myawesomelistv1.Project {
				var ps []*myawesomelistv1.Project
				for _, p := range cat.Projects {
					ps = append(ps, &myawesomelistv1.Project{
						Id:          p.ID,
						Name:        p.Name,
						Description: p.Description,
						Repo: &myawesomelistv1.Repository{
							Hostname: p.Repository.Hostname,
							Owner:    p.Repository.Owner,
							Repo:     p.Repository.Repo,
						},
						UpdatedAt: timestamppb.New(p.UpdatedAt),
					})
				}
				return ps
			}(),
		})
	}
	return pc, nil
}

// UpsertCollections stores collections in the database
func (db *Database) UpsertCollections(
	ctx context.Context,
	cols []*UpsertCollectionArgs,
) error {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.UpsertCollections")
	span.SetAttributes(attribute.Int("collections_len", len(cols)))
	defer span.End()
	if db.pg == nil {
		return fmt.Errorf("database connection not available")
	}
	repos := make([]*UpsertRepositoryArgs, 0, len(cols))
	for _, col := range cols {
		repos = append(
			repos,
			&UpsertRepositoryArgs{
				Hostname: col.Repo.Hostname,
				Owner:    col.Repo.Owner,
				Repo:     col.Repo.Repo,
			},
		)
	}
	rms, err := db.UpsertRepositories(ctx, repos)
	if err != nil {
		return err
	}
	slog.DebugContext(ctx, "upsert collections repos resolved", "count", len(rms))

	b := &pgx.Batch{}
	for i := range cols {
		b.Queue(UpsertCollectionQuery, rms[i].ID, cols[i].Language)
	}
	slog.DebugContext(ctx, "upsert collections queued", "count", len(cols))
	br := db.pg.SendBatch(ctx, b)
	defer br.Close()
	colIDs := make([]uint64, len(cols))
	for i := range cols {
		var id int64
		if err := br.QueryRow().Scan(&id); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("upsert collection failed: %w", err)
		}
		colIDs[i] = uint64(id)
	}

	for i, col := range cols {
		cats := make([]*UpsertCategoryArgs, 0, len(col.Categories))
		for _, catArg := range col.Categories {
			c := &UpsertCategoryArgs{CollectionID: colIDs[i], Name: catArg.Name}
			c.Projects = catArg.Projects
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
	embeddings [][]float32,
	limit uint32,
	repos []*myawesomelistv1.Repository,
) ([]*myawesomelistv1.Project, error) {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.SearchProjects")
	span.SetAttributes(
		attribute.Bool("embedding_used", len(embeddings) > 0),
		attribute.Int("repos_len", len(repos)),
		attribute.Int("limit", int(limit)),
	)
	defer span.End()
	if db.pg == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	var embedding *pgvector.Vector
	if len(embeddings) > 0 {
		v := pgvector.NewVector(embeddings[0])
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
		var id uint64
		var name, desc, host, owner, repo string
		var updated time.Time
		if err := rows.Scan(&id, &name, &desc, &updated, &host, &owner, &repo); err != nil {
			return nil, err
		}
		out = append(out, &myawesomelistv1.Project{
			Id:          id,
			Name:        name,
			Description: desc,
			Repo:        &myawesomelistv1.Repository{Hostname: host, Owner: owner, Repo: repo},
			UpdatedAt:   timestamppb.New(updated),
		})
	}
	slog.DebugContext(ctx, "search projects results", "count", len(out))
	return out, rows.Err()
}

// Close closes the database connection

// GetProjectStats retrieves project stats from the datastore
func (db *Database) GetProjectStats(
	ctx context.Context,
	args GetProjectStatsArgs,
) (*myawesomelistv1.ProjectStats, error) {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.GetProjectStats")
	span.SetAttributes(
		attribute.String("owner", args.Repo.Owner),
		attribute.String("repo", args.Repo.Repo),
	)
	defer span.End()
	if db.pg == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	var rid uint64
	if err := db.pg.QueryRow(ctx, RepoIDQuery, args.Repo.Hostname, args.Repo.Owner, args.Repo.Repo).Scan(&rid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to resolve repository: %w", err)
	}
	var id uint64
	var stargazers *uint32
	var openIssues *uint32
	var updated time.Time
	err := db.pg.QueryRow(ctx, ProjectStatsByRepoIDQuery, rid).
		Scan(&id, &rid, &stargazers, &openIssues, &updated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("query project stats failed: %w", err)
	}
	return &myawesomelistv1.ProjectStats{
		Id:              id,
		StargazersCount: stargazers,
		OpenIssueCount:  openIssues,
		UpdatedAt:       timestamppb.New(updated),
	}, nil
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
		var id uint64
		var stargazers *uint32
		var openIssues *uint32
		var updated time.Time
		err := db.pg.QueryRow(ctx, ProjectStatsByRepoIDQuery, rid).
			Scan(&id, &rid, &stargazers, &openIssues, &updated)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("query project stats failed: %w", err)
		}
		out = append(
			out,
			&myawesomelistv1.ProjectStats{
				Id:              id,
				StargazersCount: stargazers,
				OpenIssueCount:  openIssues,
				UpdatedAt:       timestamppb.New(updated),
			},
		)
	}
	return out, nil
}

// UpsertProjectStats stores project stats in the datastore
func (db *Database) UpsertProjectStats(
	ctx context.Context,
	args UpsertProjectStatsArgs,
) error {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.UpsertProjectStats")
	span.SetAttributes(attribute.Int("repo_id", int(args.RepositoryID)))
	defer span.End()
	slog.DebugContext(ctx, "upsert project stats", "repo_id", args.RepositoryID)
	b := &pgx.Batch{}
	b.Queue(UpsertProjectStatsQuery, args.RepositoryID, args.StargazersCount, args.OpenIssueCount)
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
	categories []*UpsertCategoryArgs,
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
		// collect generated category IDs to propagate into project args
		ids := make([]uint64, len(categories))
		for i := range categories {
			var id int64
			if err := br.QueryRow().Scan(&id); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return fmt.Errorf("upsert categories failed: %w", err)
			}
			ids[i] = uint64(id)
		}
		var projects []*UpsertProjectArgs
		for i, cm := range categories {
			for _, project := range cm.Projects {
				rms, err := db.UpsertRepositories(
					ctx,
					[]*UpsertRepositoryArgs{
						{
							Hostname: project.Repository.Hostname,
							Owner:    project.Repository.Owner,
							Repo:     project.Repository.Repo,
						},
					},
				)
				if err != nil || len(rms) == 0 {
					return fmt.Errorf("upsert project repository failed: %w", err)
				}
				projects = append(projects, &UpsertProjectArgs{
					CategoryID:   ids[i],
					RepositoryID: rms[0].ID,
					Name:         project.Name,
					Description:  project.Description,
				})
			}
		}
		if err := db.UpsertProjects(ctx, projects); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("upsert projects failed: %w", err)
		}
	}
	return nil
}

// UpsertProjects upserts projects and their embeddings
func (db *Database) UpsertProjects(
	ctx context.Context,
	projects []*UpsertProjectArgs,
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
	for range projects {
		var id int64
		if err := br.QueryRow().Scan(&id); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("upsert project failed: %w", err)
		}
		_ = uint64(id)
	}

	return nil
}

func (db *Database) UpsertProjectMetadata(
	ctx context.Context,
	args UpsertProjectMetadataArgs,
) error {
	slog.DebugContext(
		ctx,
		"upsert project metadata",
		"repo_id",
		args.RepositoryID,
		"readme_len",
		len(args.Readme),
	)
	b := &pgx.Batch{}
	b.Queue(UpsertProjectMetadataQuery, args.RepositoryID, args.Readme)
	br := db.pg.SendBatch(ctx, b)
	defer br.Close()
	if _, err := br.Exec(); err != nil {
		return fmt.Errorf("upsert project metadata failed: %w", err)
	}
	return nil
}

func (db *Database) ListStaledProjectEmbeddings(
	ctx context.Context,
	args ListStaledProjectEmbeddingsArgs,
) ([]StaledProjectEmbeddingResult, error) {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.ListStaledProjectEmbeddings")
	defer span.End()
	if db.pg == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	ttlSeconds := int64(args.TTL.Seconds())
	pr, err := db.pg.Query(ctx, ProjectsStaledEmbeddingsQuery, ttlSeconds)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("list staled project embeddings query failed: %w", err)
	}
	defer pr.Close()
	rows, err := pgx.CollectRows(pr, pgx.RowToStructByPos[StaledProjectEmbeddingResult])
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return rows, nil
}

func (db *Database) UpsertProjectEmbedding(
	ctx context.Context,
	args UpsertProjectEmbeddingArgs,
) error {
	tracer := otel.Tracer("myawesomelist/database")
	ctx, span := tracer.Start(ctx, "Database.UpsertProjectEmbedding")
	span.SetAttributes(attribute.Int("vector_dim", len(args.Vec)))
	defer span.End()
	if db.pg == nil {
		return fmt.Errorf("database connection not available")
	}
	v := pgvector.NewVector(args.Vec)
	if _, err := db.pg.Exec(ctx, UpsertProjectEmbeddingQuery, args.ProjectID, v); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("upsert project embedding failed: %w", err)
	}
	return nil
}
