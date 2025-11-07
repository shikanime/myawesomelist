SELECT stargazers_count, open_issue_count, updated_at
FROM project_stats
WHERE hostname = $1 AND owner = $2 AND repo = $3