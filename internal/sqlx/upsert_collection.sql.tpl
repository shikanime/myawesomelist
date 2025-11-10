INSERT INTO collections (hostname, owner, repo, language)
VALUES ($1, $2, $3, $4)
ON CONFLICT (hostname, owner, repo)
DO UPDATE SET
    language = EXCLUDED.language,
    updated_at = NOW()
RETURNING id