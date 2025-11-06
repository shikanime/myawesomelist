-- Create a dedicated table to cache per-project GitHub stats
CREATE TABLE IF NOT EXISTS project_stats (
    id SERIAL PRIMARY KEY,
    owner VARCHAR(255) NOT NULL,
    repo VARCHAR(255) NOT NULL,
    stargazers_count INTEGER,
    open_issue_count INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(owner, repo)
);

-- Indexes to speed up lookups
CREATE INDEX IF NOT EXISTS idx_project_stats_owner_repo ON project_stats(owner, repo);
CREATE INDEX IF NOT EXISTS idx_project_stats_updated_at ON project_stats(updated_at);

-- Reuse the trigger function to auto-refresh updated_at
CREATE TRIGGER update_project_stats_updated_at
    BEFORE UPDATE ON project_stats
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();