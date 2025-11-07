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

func UpSertCollectionQuery(
	repo *myawesomelistv1.Repository,
	col *myawesomelistv1.Collection,
) (string, []any, error) {
	q, err := sqlTemplates.ReadFile("upsert_collection.sql.tpl")
	if err != nil {
		return "", nil, fmt.Errorf("read sql template upsert_collection.sql.tpl: %w", err)
	}
	return string(q), []any{repo.Hostname, repo.Owner, repo.Repo, col.Language}, nil
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
) (string, []any, error) {
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
		return "", nil, fmt.Errorf("read sql template search_projects.sql.tpl: %w", err)
	}
	if err := sqlQuery.Execute(&buf, params); err != nil {
		return "", nil, err
	}

	return buf.String(), SearchProjectsArgs(q, repos, limit), nil
}

// GetProjectStatsQuery returns project stats for a repository.
func GetProjectStatsQuery(repo *myawesomelistv1.Repository) (string, []any, error) {
	q, err := sqlTemplates.ReadFile("get_project_stats.sql.tpl")
	if err != nil {
		return "", nil, fmt.Errorf("read sql template get_project_stats.sql.tpl: %w", err)
	}
	return string(q), []any{repo.Hostname, repo.Owner, repo.Repo}, nil
}

// UpSertProjectStatsQuery upserts project stats for a repository.
func UpSertProjectStatsQuery(
	repo *myawesomelistv1.Repository,
	stats *myawesomelistv1.ProjectStats,
) (string, []any, error) {
	q, err := sqlTemplates.ReadFile("upsert_project_stats.sql.tpl")
	if err != nil {
		return "", nil, fmt.Errorf("read sql template upsert_project_stats.sql.tpl: %w", err)
	}
	return string(q), []any{
		repo.Hostname,
		repo.Owner,
		repo.Repo,
		stats.StargazersCount,
		stats.OpenIssueCount,
	}, nil
}

// UpSertCategoryQuery upserts a category for a collection and returns its id.
func UpSertCategoryQuery(
	collectionID int64,
	category *myawesomelistv1.Category,
) (string, []any, error) {
	if category == nil || category.Name == "" {
		return "", nil, fmt.Errorf("invalid category")
	}
	q, err := sqlTemplates.ReadFile("upsert_category.sql.tpl")
	if err != nil {
		return "", nil, fmt.Errorf("read sql template upsert_category.sql.tpl: %w", err)
	}
	return string(q), []any{collectionID, category.Name}, nil
}

// UpSertProjectQuery renders a single-project upsert statement using category_id.
func UpSertProjectQuery(
	categoryID int64,
	project *myawesomelistv1.Project,
) (string, []any, error) {
	if project == nil || project.Repo == nil {
		return "", nil, fmt.Errorf("invalid project or missing repo")
	}
	q, err := sqlTemplates.ReadFile("upsert_project.sql.tpl")
	if err != nil {
		return "", nil, fmt.Errorf("read sql template upsert_project.sql.tpl: %w", err)
	}
	return string(
			q,
		), []any{
			categoryID,
			project.Name,
			project.Description,
			project.Repo.Hostname,
			project.Repo.Owner,
			project.Repo.Repo,
		}, nil
}

// GetCategoriesQuery returns all categories for a collection.
func GetCategoriesQuery(collectionID int64) (string, []any, error) {
	q, err := sqlTemplates.ReadFile("get_categories.sql.tpl")
	if err != nil {
		return "", nil, fmt.Errorf("read sql template get_categories.sql.tpl: %w", err)
	}
	return string(q), []any{collectionID}, nil
}

// GetProjectsQuery returns all projects for a category.
func GetProjectsQuery(categoryID int64) (string, []any, error) {
	q, err := sqlTemplates.ReadFile("get_projects.sql.tpl")
	if err != nil {
		return "", nil, fmt.Errorf("read sql template get_projects.sql.tpl: %w", err)
	}
	return string(q), []any{categoryID}, nil
}
