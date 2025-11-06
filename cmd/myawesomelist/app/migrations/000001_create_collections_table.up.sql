-- Create the collections table to cache curated repo data and metadata
CREATE TABLE IF NOT EXISTS collections (
    id SERIAL PRIMARY KEY,
    hostname VARCHAR(255) NOT NULL,
    owner VARCHAR(255) NOT NULL,
    repo VARCHAR(255) NOT NULL,
    language VARCHAR(100) NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(hostname, owner, repo)
);

-- Indexes to speed up common queries (lookup, sort, filter)
CREATE INDEX IF NOT EXISTS idx_collections_hostname_owner_repo ON collections(hostname, owner, repo);
CREATE INDEX IF NOT EXISTS idx_collections_updated_at ON collections(updated_at);
CREATE INDEX IF NOT EXISTS idx_collections_language ON collections(language);

-- Trigger function to automatically refresh updated_at on row updates
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger that applies the updated_at refresh logic before each row update
CREATE TRIGGER update_collections_updated_at
    BEFORE UPDATE ON collections
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
