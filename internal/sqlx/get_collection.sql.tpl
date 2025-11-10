SELECT id, language, updated_at
FROM collections
WHERE hostname = $1 AND owner = $2 AND repo = $3