INSERT INTO project_stats (hostname, owner, repo, stargazers_count, open_issue_count)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (hostname, owner, repo)
DO UPDATE SET
    stargazers_count = EXCLUDED.stargazers_count,
    open_issue_count = EXCLUDED.open_issue_count