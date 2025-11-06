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

func UpsertCollectionQuery(repo *myawesomelistv1.Repository, col *myawesomelistv1.Collection) (string, []any, error) {
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
        p->>'name' AS name,
        p->>'description' AS description,
        p->>'url' AS url,
        NULLIF(p->>'stargazers_count','')::int AS stargazers,
        NULLIF(p->>'open_issue_count','')::int AS open_issues
    FROM collections c
    JOIN LATERAL jsonb_array_elements((c.data::jsonb)->'categories') cat ON TRUE
    JOIN LATERAL jsonb_array_elements(cat->'projects') p ON TRUE
    WHERE
        (LOWER(p->>'name') LIKE $1 OR LOWER(p->>'description') LIKE $1 OR LOWER(p->>'url') LIKE $1)
    {{- if .RepoPositions }}
      AND (
        {{- range $i, $pos := .RepoPositions -}}
          {{- if $i }} OR {{ end -}}
          (c.hostname = ${{ $pos.HostnamePosition }} AND c.owner = ${{ $pos.OwnerPosition }} AND c.repo = ${{ $pos.RepoPosition }})
        {{- end -}}
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
func SearchProjectsQuery(q string, repos []*myawesomelistv1.Repository, limit uint32) (string, []any, error) {
	repoPositions := make([]SearchRepoPos, 0, len(repos))
	for i := range repos {
		hostnamePos := i*3 + 1
		ownerPos := i*3 + 2
		repoPos := i*3 + 3
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

func UpsertProjectStatsQuery(repo *myawesomelistv1.Repository, stats *myawesomelistv1.ProjectsStats) (string, []any, error) {
	query := `
        INSERT INTO project_stats (hostname, owner, repo, stargazers_count, open_issue_count)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (hostname, owner, repo)
        DO UPDATE SET
            stargazers_count = EXCLUDED.stargazers_count,
            open_issue_count = EXCLUDED.open_issue_count
    `
	return query, []any{repo.Hostname, repo.Owner, repo.Repo, stats.StargazersCount, stats.OpenIssueCount}, nil
}
