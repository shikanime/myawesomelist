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
    description = EXCLUDED.description,
    updated_at = NOW()