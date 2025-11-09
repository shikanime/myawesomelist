SELECT id, language
FROM collections
WHERE hostname = $1 AND owner = $2 AND repo = $3