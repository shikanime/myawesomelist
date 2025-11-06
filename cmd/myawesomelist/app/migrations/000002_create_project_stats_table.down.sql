-- Drop trigger before dropping table
DROP TRIGGER IF EXISTS update_project_stats_updated_at ON project_stats;

-- Drop indexes
DROP INDEX IF EXISTS idx_project_stats_owner_repo;
DROP INDEX IF EXISTS idx_project_stats_updated_at;

-- Drop table
DROP TABLE IF EXISTS project_stats;