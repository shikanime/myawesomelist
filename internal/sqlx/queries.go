package sqlx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

func GetCollectionQuery(repo *myawesomelistv1.Repository) (string, []any, error) {
	query := `
		SELECT language, data, updated_at
		FROM collections
		WHERE hostname = $1 AND owner = $2 AND repo = $3
	`
	return query, []any{repo.Hostname, repo.Owner, repo.Repo}, nil
}

func UpSertCollectionQuery(
	repo *myawesomelistv1.Repository,
	col *myawesomelistv1.Collection,
) (string, []any, error) {
	data, err := json.Marshal(col)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal collection: %w", err)
	}
	query := `
		INSERT INTO collections (hostname, owner, repo, language, data)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (hostname, owner, repo)
		DO UPDATE SET
			language = EXCLUDED.language,
			data = EXCLUDED.data
		RETURNING id
	`
	return query, []any{repo.Hostname, repo.Owner, repo.Repo, col.Language, data}, nil
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

var searchProjectsTpl = template.Must(template.New("searchProjects").Parse(`
	SELECT
		COALESCE(p.name,'') AS name,
		COALESCE(p.description,'') AS description,
		p.repo_hostname AS hostname,
		p.repo_owner AS owner,
		p.repo_repo AS repo
	FROM projects p
	JOIN categories cat ON cat.id = p.category_id
	JOIN collections c ON c.id = cat.collection_id
	WHERE
		(LOWER(p.name) LIKE $1 OR LOWER(p.description) LIKE $1)
	{{- if .RepoPositions }}
		AND (
			{{- range $i, $pos := .RepoPositions }}
			{{- if $i }} OR {{ end }}
			(c.hostname = ${{ $pos.HostnamePosition }} AND c.owner = ${{ $pos.OwnerPosition }} AND c.repo = ${{ $pos.RepoPosition }})
			{{- end }}
   		)
	{{- end }}
	LIMIT ${{ .LimitPosition }}
`))

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
	repoPositions := make([]SearchRepoPos, 0, len(repos))
	for i := range repos {
		// pattern is $1, so repo positions start at $2
		hostnamePos := i*3 + 2
		ownerPos := i*3 + 3
		repoPos := i*3 + 4
		repoPositions = append(repoPositions, SearchRepoPos{
			HostnamePosition: hostnamePos,
			OwnerPosition:    ownerPos,
			RepoPosition:     repoPos,
		})
	}
	limitPos := 3*len(repos) + 2

	params := SearchProjectsParams{
		RepoPositions: repoPositions,
		LimitPosition: limitPos,
	}
	var buf bytes.Buffer
	if err := searchProjectsTpl.Execute(&buf, params); err != nil {
		return "", nil, err
	}

	return buf.String(), SearchProjectsArgs(q, repos, limit), nil
}

func GetProjectStatsQuery(repo *myawesomelistv1.Repository) (string, []any, error) {
	query := `
        SELECT stargazers_count, open_issue_count, updated_at
        FROM project_stats
        WHERE hostname = $1 AND owner = $2 AND repo = $3
    `
	return query, []any{repo.Hostname, repo.Owner, repo.Repo}, nil
}

func UpSertProjectStatsQuery(
	repo *myawesomelistv1.Repository,
	stats *myawesomelistv1.ProjectStats,
) (string, []any, error) {
	query := `
        INSERT INTO project_stats (hostname, owner, repo, stargazers_count, open_issue_count)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (hostname, owner, repo)
        DO UPDATE SET
            stargazers_count = EXCLUDED.stargazers_count,
            open_issue_count = EXCLUDED.open_issue_count
    `
	return query, []any{
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
	cat *myawesomelistv1.Category,
) (string, []any, error) {
	if cat == nil || cat.Name == "" {
		return "", nil, fmt.Errorf("invalid category")
	}
	query := `
		INSERT INTO categories (collection_id, name)
		VALUES ($1, $2)
		ON CONFLICT (collection_id, name)
		DO UPDATE SET
			name = EXCLUDED.name
		RETURNING id
	`
	return query, []any{collectionID, cat.Name}, nil
}

// UpSertProjectQuery renders a single-project upsert statement using category_id.
func UpSertProjectQuery(
	categoryID int64,
	p *myawesomelistv1.Project,
) (string, []any, error) {
	if p == nil || p.Repo == nil {
		return "", nil, fmt.Errorf("invalid project or missing repo")
	}
	query := `
		INSERT INTO projects (
			category_id,
			name, description,
			repo_hostname, repo_owner, repo_repo
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (
			category_id,
			repo_hostname, repo_owner, repo_repo
		)
		DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description
	`
	return query, []any{
		categoryID,
		p.Name,
		p.Description,
		p.Repo.Hostname,
		p.Repo.Owner,
		p.Repo.Repo,
	}, nil
}
