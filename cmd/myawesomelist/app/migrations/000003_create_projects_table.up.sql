-- Create normalized hierarchy: collections -> categories -> projects
-- Categories table to normalize projects under collections
CREATE TABLE IF NOT EXISTS categories (
    id SERIAL PRIMARY KEY,

    -- Parent collection relation
    collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,

    -- Category identity
    name VARCHAR(255) NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Prevent duplicates within a collection
    UNIQUE (collection_id, name)
);

-- Helpful indexes
CREATE INDEX IF NOT EXISTS idx_categories_collection ON categories(collection_id);
CREATE INDEX IF NOT EXISTS idx_categories_name ON categories(name);

-- Projects table uses category_id instead of collection_id
CREATE TABLE IF NOT EXISTS projects (
    id SERIAL PRIMARY KEY,

    -- Parent category relation
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,

    -- Project identity and description
    name             VARCHAR(255) NOT NULL,
    description      TEXT,

    -- Target repository this project links to
    repo_hostname    VARCHAR(255) NOT NULL,
    repo_owner       VARCHAR(255) NOT NULL,
    repo_repo        VARCHAR(255) NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Prevent duplicates when re-parsing within a category
    UNIQUE (
        category_id,
        repo_hostname, repo_owner, repo_repo
    )
);

-- Helpful indexes
CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);
CREATE INDEX IF NOT EXISTS idx_projects_category ON projects(category_id);