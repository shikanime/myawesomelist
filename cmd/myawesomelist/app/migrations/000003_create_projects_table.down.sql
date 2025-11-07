DROP TRIGGER IF EXISTS update_projects_updated_at ON projects;
DROP TRIGGER IF EXISTS update_categories_updated_at ON categories;

DROP INDEX IF EXISTS idx_projects_name;
DROP INDEX IF EXISTS idx_projects_category;
DROP INDEX IF EXISTS idx_categories_collection;
DROP INDEX IF EXISTS idx_categories_name;

DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS categories;