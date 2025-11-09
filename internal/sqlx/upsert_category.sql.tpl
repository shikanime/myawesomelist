INSERT INTO categories (collection_id, name)
VALUES ($1, $2)
ON CONFLICT (collection_id, name)
DO UPDATE SET
    name = EXCLUDED.name
RETURNING id