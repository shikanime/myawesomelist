package database

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/pgvector/pgvector-go"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

type UpsertRepositoriesResult struct {
	ID       uint64
	Hostname string
	Owner    string
	Repo     string
}

type UpsertRepositoryArgs struct {
	Hostname string
	Owner    string
	Repo     string
}

type CategoryProjectArg struct {
	Repository  myawesomelistv1.Repository
	Name        string
	Description string
}

type UpsertCategoryArgs struct {
	CollectionID uint64
	Name         string
	Projects     []CategoryProjectArg
}

type UpsertProjectArgs struct {
	CategoryID   uint64
	RepositoryID uint64
	Name         string
	Description  string
}

type UpsertCollectionArgs struct {
    Repo       myawesomelistv1.Repository
    Language   string
    Categories []UpsertCategoryArgs
}

type ListCollectionsArgs struct {
    Repos []*myawesomelistv1.Repository
}

type StaledProjectEmbeddingResult struct {
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

type UpsertProjectEmbeddingArgs struct {
	ProjectID uint64
	Vec       []float32
}

type ListStaledProjectEmbeddingsArgs struct {
	TTL time.Duration
}

type UpsertProjectMetadataArgs struct {
	RepositoryID uint64
	Readme       string
}

type GetProjectStatsArgs struct {
	Repo myawesomelistv1.Repository
}

type UpsertProjectStatsArgs struct {
	RepositoryID    uint64
	StargazersCount *uint32
	OpenIssueCount  *uint32
}

var UpsertRepositoryQuery = strings.Join([]string{
	"INSERT INTO repositories (hostname, owner, repo)",
	"VALUES ($1, $2, $3)",
	"ON CONFLICT (hostname, owner, repo)",
	"DO UPDATE SET updated_at = NOW()",
	"RETURNING id",
}, " ")

var UpsertCollectionQuery = strings.Join([]string{
	"INSERT INTO collections (repository_id, language)",
	"VALUES ($1, $2)",
	"ON CONFLICT (repository_id)",
	"DO UPDATE SET language = EXCLUDED.language, updated_at = NOW()",
	"RETURNING id",
}, " ")

var UpsertCategoryQuery = strings.Join([]string{
	"INSERT INTO categories (collection_id, name)",
	"VALUES ($1, $2)",
	"ON CONFLICT (collection_id, name)",
	"DO UPDATE SET updated_at = NOW()",
	"RETURNING id",
}, " ")

var UpsertProjectQuery = strings.Join([]string{
	"INSERT INTO projects (category_id, repository_id, name, description)",
	"VALUES ($1, $2, $3, $4)",
	"ON CONFLICT (category_id, repository_id)",
	"DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, updated_at = NOW()",
	"RETURNING id",
}, " ")

var UpsertProjectEmbeddingQuery = strings.Join([]string{
	"INSERT INTO project_embeddings (project_id, embedding)",
	"VALUES ($1, $2)",
	"ON CONFLICT (project_id)",
	"DO UPDATE SET embedding = EXCLUDED.embedding, updated_at = NOW()",
}, " ")

var UpsertProjectStatsQuery = strings.Join([]string{
	"INSERT INTO project_stats (repository_id, stargazers_count, open_issue_count)",
	"VALUES ($1, $2, $3)",
	"ON CONFLICT (repository_id)",
	"DO UPDATE SET stargazers_count = EXCLUDED.stargazers_count, open_issue_count = EXCLUDED.open_issue_count, updated_at = NOW()",
}, " ")

var UpsertProjectMetadataQuery = strings.Join([]string{
	"INSERT INTO project_metadata (repository_id, readme)",
	"VALUES ($1, $2)",
	"ON CONFLICT (repository_id)",
	"DO UPDATE SET readme = EXCLUDED.readme, updated_at = NOW()",
}, " ")

var RepoIDQuery = strings.Join([]string{
	"SELECT id FROM repositories",
	"WHERE hostname=$1 AND owner=$2 AND repo=$3",
}, " ")

var CollectionByRepoIDQuery = strings.Join([]string{
	"SELECT c.id, c.repository_id, c.language, c.updated_at,",
	"r.hostname, r.owner, r.repo",
	"FROM collections c JOIN repositories r ON r.id=c.repository_id",
	"WHERE c.repository_id=$1",
}, " ")

var CategoriesByCollectionIDsQuery = strings.Join([]string{
	"SELECT id, collection_id, name, updated_at",
	"FROM categories",
	"WHERE collection_id = ANY($1::bigint[])",
}, " ")

var ProjectsByCategoryIDsQuery = strings.Join([]string{
	"SELECT p.id, p.category_id, p.repository_id, p.name, p.description, p.updated_at,",
	"r.hostname, r.owner, r.repo FROM projects p JOIN repositories r ON r.id = p.repository_id",
	"WHERE p.category_id = ANY($1::bigint[])",
}, " ")

var ProjectsStaledEmbeddingsQuery = strings.Join([]string{
	"SELECT p.id, p.category_id, p.repository_id, p.name, p.description, p.updated_at,",
	"r.hostname, r.owner, r.repo FROM projects p",
	"JOIN repositories r ON r.id = p.repository_id",
	"LEFT JOIN project_embeddings pe ON pe.project_id = p.id",
	"WHERE pe.updated_at IS NULL",
	"OR ($1::double precision >= 0 AND EXTRACT(EPOCH FROM NOW() - pe.updated_at) > $1::double precision)",
}, " ")

var ProjectStatsByRepoIDQuery = strings.Join([]string{
	"SELECT id, repository_id, stargazers_count, open_issue_count, updated_at",
	"FROM project_stats",
	"WHERE repository_id=$1",
}, " ")

var tmplFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"mul": func(a, b int) int { return a * b },
}

var listCollectionsQueryTmpl = template.Must(
	template.New("listCollections").Funcs(tmplFuncs).Parse(strings.Join([]string{
		"SELECT c.id, c.repository_id, c.language, c.updated_at, r.hostname, r.owner, r.repo",
		"FROM collections c",
		"JOIN repositories r ON r.id = c.repository_id",
		"{{if gt (len .Repos) 0}}",
		"WHERE {{range $i, $rp := .Repos}}{{if ne $i 0}} OR {{end}}(r.hostname = ${{add (mul $i 3) 1}} AND r.owner = ${{add (mul $i 3) 2}} AND r.repo = ${{add (mul $i 3) 3}}){{end}}",
		"{{end}}",
	}, " ")),
)

var searchProjectsQueryTmpl = template.Must(
	template.New("searchProjects").Funcs(tmplFuncs).Parse(strings.Join([]string{
		"SELECT p.id, p.name, p.description, p.updated_at, r.hostname, r.owner, r.repo",
		"FROM projects p",
		"JOIN repositories r ON r.id = p.repository_id",
		"JOIN project_embeddings pe ON pe.project_id = p.id",
		"{{if gt (len .Repos) 0}} WHERE {{range $i, $rp := .Repos}}{{if ne $i 0}} OR {{end}}(r.hostname = ${{add (mul $i 3) 1}} AND r.owner = ${{add (mul $i 3) 2}} AND r.repo = ${{add (mul $i 3) 3}}){{end}}{{end}}",
		"{{if .OrderPlaceholder}} ORDER BY pe.embedding <-> {{.OrderPlaceholder}}{{end}}",
		"{{if .LimitPlaceholder}} LIMIT {{.LimitPlaceholder}}{{end}}",
	}, " ")),
)

// RenderListCollectionsArgs renders positional arguments for list collections query given repos
func RenderListCollectionsArgs(repos []*myawesomelistv1.Repository) []any {
	args := make([]any, 0, len(repos)*3)
	for _, rp := range repos {
		args = append(args, rp.Hostname, rp.Owner, rp.Repo)
	}
	return args
}

// RenderListCollectionsQuery builds SQL and args for listing collections filtered by repositories
func RenderListCollectionsQuery(repos []*myawesomelistv1.Repository) (string, []any, error) {
	args := RenderListCollectionsArgs(repos)
	var buf bytes.Buffer
	if err := listCollectionsQueryTmpl.Execute(&buf, map[string]interface{}{"Repos": repos}); err != nil {
		return "", nil, err
	}
	return buf.String(), args, nil
}

// RenderSearchProjectsArgs renders positional arguments and placeholders for search projects query
func RenderSearchProjectsArgs(
	repos []*myawesomelistv1.Repository,
	embedding *pgvector.Vector,
	limit int,
) ([]any, string, string) {
	args := RenderListCollectionsArgs(repos)
	idx := 1 + len(repos)*3
	var orderPlaceholder string
	if embedding != nil {
		orderPlaceholder = fmt.Sprintf("$%d", idx)
		args = append(args, *embedding)
		idx++
	}
	limitPlaceholder := fmt.Sprintf("$%d", idx)
	args = append(args, limit)
	return args, orderPlaceholder, limitPlaceholder
}

// RenderSearchProjectsQuery builds SQL and args for searching projects filtered by repositories.
// If embedding is non-nil, an ORDER BY clause on embedding distance is added and the embedding is appended to args.
func RenderSearchProjectsQuery(
	repos []*myawesomelistv1.Repository,
	embedding *pgvector.Vector,
	limit int,
) (string, []any, error) {
	args, orderPlaceholder, limitPlaceholder := RenderSearchProjectsArgs(repos, embedding, limit)
	var buf bytes.Buffer
	if err := searchProjectsQueryTmpl.Execute(&buf, map[string]interface{}{"Repos": repos, "OrderPlaceholder": orderPlaceholder, "LimitPlaceholder": limitPlaceholder}); err != nil {
		return "", nil, err
	}
	return buf.String(), args, nil
}
