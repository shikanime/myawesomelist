SELECT name, description, repo_hostname, repo_owner, repo_repo, updated_at
FROM projects
WHERE category_id = $1