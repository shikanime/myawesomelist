-- Drop trigger before dropping its function/table
DROP TRIGGER IF EXISTS update_project_stats_updated_at ON project_stats;

-- Explicitly drop indexes (also dropped with table, but clearer here)
DROP INDEX IF EXISTS idx_project_stats_hostname_owner_repo;
DROP INDEX IF EXISTS idx_project_stats_updated_at;

-- Drop table
DROP TABLE IF EXISTS project_stats;