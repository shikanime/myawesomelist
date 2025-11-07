SELECT name, description, repo_hostname, repo_owner, repo_repo
FROM projects
WHERE category_id = $1