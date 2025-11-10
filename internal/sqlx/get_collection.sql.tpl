SELECT
    cat.id AS category_id,
    cat.name AS category_name,
    cat.updated_at AS category_updated_at,
    p.name AS project_name,
    p.description AS project_description,
    p.repo_hostname AS project_repo_hostname,
    p.repo_owner AS project_repo_owner,
    p.repo_repo AS project_repo_repo,
    p.updated_at AS project_updated_at,
    c.language AS collection_language,
    c.updated_at AS collection_updated_at
FROM collections c
LEFT JOIN categories cat ON cat.collection_id = c.id
LEFT JOIN projects p ON p.category_id = cat.id
WHERE c.hostname = $1 AND c.owner = $2 AND c.repo = $3