-- Drop trigger before dropping its function/table
DROP TRIGGER IF EXISTS update_collections_updated_at ON collections;

-- Remove the helper function used by the trigger
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Explicitly drop indexes (also dropped with table, but clearer here)
DROP INDEX IF EXISTS idx_collections_hostname_owner_repo;
DROP INDEX IF EXISTS idx_collections_updated_at;
DROP INDEX IF EXISTS idx_collections_language;

-- Finally, drop the collections table
DROP TABLE IF EXISTS collections;
