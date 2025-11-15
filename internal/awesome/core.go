package awesome

import (
	"context"

	"myawesomelist.shikanime.studio/internal/database"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// Core provides core, datastore-backed operations independent of external APIs.
type Core struct {
	db *database.Database
}

// NewCoreClient constructs a Core using the provided datastore.
func NewCoreClient(db *database.Database) *Core {
	return &Core{db: db}
}

// SearchProjects executes a datastore-backed search across repositories using Core.
func (c *Core) SearchProjects(
	ctx context.Context,
	q string,
	limit uint32,
	repos []*myawesomelistv1.Repository,
) ([]*myawesomelistv1.Project, error) {
	return c.db.SearchProjects(ctx, q, limit, repos)
}
