package core

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"myawesomelist.shikanime.studio/internal/agent"
	"myawesomelist.shikanime.studio/internal/database"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

type Agent struct {
	db  *database.Database
	emb *agent.Embeddings
}

func NewAgentClient(db *database.Database, emb *agent.Embeddings) *Agent {
	return &Agent{db: db, emb: emb}
}

func (c *Agent) SearchProjects(
	ctx context.Context,
	req *myawesomelistv1.SearchProjectsRequest,
) ([]*myawesomelistv1.Project, error) {
	tracer := otel.Tracer("myawesomelist/agent")
	ctx, span := tracer.Start(ctx, "Agent.SearchProjects")
	q := req.GetQuery()
	limit := req.GetLimit()
	repos := req.GetRepos()
	span.SetAttributes(
		attribute.String("query", q),
		attribute.Int("repos_len", len(repos)),
		attribute.Int("limit", int(limit)),
	)
	defer span.End()
	var vecs [][]float32
	if q != "" {
		v, err := c.emb.EmbedProjects(
			ctx,
			[]*myawesomelistv1.Project{{Name: q, Description: q}},
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		vecs = v
	}
	out, err := c.db.SearchProjects(ctx, vecs, limit, repos)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return out, nil
}

func (c *Agent) UpsertAllStaledProjectEmbeddings(
	ctx context.Context,
	ttl time.Duration,
) error {
	tracer := otel.Tracer("myawesomelist/agent")
	ctx, span := tracer.Start(ctx, "Agent.UpsertAllStaledProjectEmbeddings")
	span.SetAttributes(attribute.Int("ttl_seconds", int(ttl.Seconds())))
	defer span.End()
	pes, err := c.db.ListStaledProjectEmbeddings(
		ctx,
		database.ListStaledProjectEmbeddingsArgs{TTL: ttl},
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if len(pes) == 0 {
		return nil
	}
	inputs := make([]*myawesomelistv1.Project, len(pes))
	for i := range pes {
		inputs[i] = &myawesomelistv1.Project{
			Id:          pes[i].ID,
			Name:        pes[i].Name,
			Description: pes[i].Description,
			Repo: &myawesomelistv1.Repository{
				Hostname: pes[i].Hostname,
				Owner:    pes[i].Owner,
				Repo:     pes[i].Repo,
			},
		}
	}
	vecs, err := c.emb.EmbedProjects(ctx, inputs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	for i := range pes {
		if err := c.db.UpsertProjectEmbedding(ctx, database.UpsertProjectEmbeddingArgs{ProjectID: pes[i].ID, Vec: vecs[i]}); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	}
	return nil
}
