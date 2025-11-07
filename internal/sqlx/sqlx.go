package sqlx

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

//go:embed *.sql.tpl
var sqlTemplates embed.FS

func GetCollectionQuery(repo *myawesomelistv1.Repository) (string, []any, error) {
	q, err := sqlTemplates.ReadFile("get_collection.sql.tpl")
	if err != nil {
		return "", nil, fmt.Errorf("read sql template get_collection.sql.tpl: %w", err)
	}
	return string(q), []any{repo.Hostname, repo.Owner, repo.Repo}, nil
}

func UpSertCollectionArgs(
	repo *myawesomelistv1.Repository,
	col *myawesomelistv1.Collection,
) []any {
	return []any{repo.Hostname, repo.Owner, repo.Repo, col.Language}
}

// UpSertCollectionQuery builds the upsert collection SQL and args.
func UpSertCollectionQuery() (string, error) {
	q, err := sqlTemplates.ReadFile("upsert_collection.sql.tpl")
	if err != nil {
		return "", fmt.Errorf("read sql template upsert_collection.sql.tpl: %w", err)
	}
	return string(q), nil
}

type SearchRepoPos struct {
	HostnamePosition int
	OwnerPosition    int
	RepoPosition     int
}

type SearchProjectsParams struct {
	RepoPositions []SearchRepoPos
	LimitPosition int
}

// SearchProjectsArgs builds the args: pattern, [hostname, owner, repo]*, limit
func SearchProjectsArgs(q string, repos []*myawesomelistv1.Repository, limit uint32) []any {
	args := []any{"%" + q + "%"}
	for i := range repos {
		args = append(args, repos[i].Hostname, repos[i].Owner, repos[i].Repo)
	}
	return append(args, limit)
}

// SearchProjectsQuery builds the SQL query for project search.
func SearchProjectsQuery(
	q string,
	repos []*myawesomelistv1.Repository,
	limit uint32,
) (string, error) {
	repoPoss := make([]SearchRepoPos, 0, len(repos))
	for i := range repos {
		// pattern is $1, so repo positions start at $2
		hostnamePos := i*3 + 2
		ownerPos := i*3 + 3
		repoPos := i*3 + 4
		repoPoss = append(repoPoss, SearchRepoPos{
			HostnamePosition: hostnamePos,
			OwnerPosition:    ownerPos,
			RepoPosition:     repoPos,
		})
	}
	limitPos := 3*len(repos) + 2

	params := SearchProjectsParams{
		RepoPositions: repoPoss,
		LimitPosition: limitPos,
	}
	var buf bytes.Buffer
	sqlQuery, err := template.ParseFS(sqlTemplates, "search_projects.sql.tpl")
	if err != nil {
		return "", fmt.Errorf("read sql template search_projects.sql.tpl: %w", err)
	}
	if err := sqlQuery.Execute(&buf, params); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func GetProjectStatsArgs(repo *myawesomelistv1.Repository) []any {
	return []any{repo.Hostname, repo.Owner, repo.Repo}
}

// GetProjectStatsQuery returns project stats for a repository.
func GetProjectStatsQuery() (string, error) {
	q, err := sqlTemplates.ReadFile("get_project_stats.sql.tpl")
	if err != nil {
		return "", fmt.Errorf("read sql template get_project_stats.sql.tpl: %w", err)
	}
	return string(q), nil
}

// GetCategoriesArgs returns the args for get_categories.sql.tpl.
func GetCategoriesArgs(collectionID int64) []any {
	return []any{collectionID}
}

// GetCategoriesQuery returns all categories for a collection.
func GetCategoriesQuery() (string, error) {
	q, err := sqlTemplates.ReadFile("get_categories.sql.tpl")
	if err != nil {
		return "", fmt.Errorf("read sql template get_categories.sql.tpl: %w", err)
	}
	return string(q), nil
}

// GetProjectsArgs returns the args for get_projects.sql.tpl.
func GetProjectsArgs(categoryID int64) []any {
	return []any{categoryID}
}

// GetProjectsQuery returns all projects for a category.
func GetProjectsQuery() (string, error) {
	q, err := sqlTemplates.ReadFile("get_projects.sql.tpl")
	if err != nil {
		return "", fmt.Errorf("read sql template get_projects.sql.tpl: %w", err)
	}
	return string(q), nil
}

func UpSertProjectStatsArgs(
	repo *myawesomelistv1.Repository,
	stats *myawesomelistv1.ProjectStats,
) []any {
	return []any{
		repo.Hostname,
		repo.Owner,
		repo.Repo,
		stats.StargazersCount,
		stats.OpenIssueCount,
	}
}

// UpSertProjectStatsQuery upserts project stats for a repository.
func UpSertProjectStatsQuery() (string, error) {
	q, err := sqlTemplates.ReadFile("upsert_project_stats.sql.tpl")
	if err != nil {
		return "", fmt.Errorf("read sql template upsert_project_stats.sql.tpl: %w", err)
	}
	return string(q), nil
}

func UpSertCategoryArgs(
	collectionID int64,
	category *myawesomelistv1.Category,
) []any {
	return []any{collectionID, category.Name}
}

// UpSertCategoryQuery upserts a category for a collection and returns its id.
func UpSertCategoryQuery() (string, error) {
	q, err := sqlTemplates.ReadFile("upsert_category.sql.tpl")
	if err != nil {
		return "", fmt.Errorf("read sql template upsert_category.sql.tpl: %w", err)
	}
	return string(q), nil
}

func UpSertProjectArgs(
	categoryID int64,
	project *myawesomelistv1.Project,
) []any {
	return []any{
		categoryID,
		project.Name,
		project.Description,
		project.Repo.Hostname,
		project.Repo.Owner,
		project.Repo.Repo,
	}
}

// UpSertProjectQuery renders a single-project upsert statement using category_id.
func UpSertProjectQuery() (string, error) {
	q, err := sqlTemplates.ReadFile("upsert_project.sql.tpl")
	if err != nil {
		return "", fmt.Errorf("read sql template upsert_project.sql.tpl: %w", err)
	}
	return string(q), nil
}
