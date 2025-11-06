package awesome

import (
	"context"

	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// Core provides core, datastore-backed operations independent of external APIs.
type Core struct {
	ds *DataStore
}

// NewCoreClient constructs a Core using the provided datastore.
func NewCoreClient(ds *DataStore) *Core {
	return &Core{ds: ds}
}

// SearchProjects executes a datastore-backed search across repositories using Core.
func (c *Core) SearchProjects(
	ctx context.Context,
	q string,
	limit uint32,
	repos []*myawesomelistv1.Repository,
) ([]*myawesomelistv1.Project, error) {
	return c.ds.SearchProjects(ctx, q, limit, repos)
}
