package grpc

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"myawesomelist.shikanime.studio/internal/awesome"
	"myawesomelist.shikanime.studio/internal/awesome/github"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
	myawesomelistv1connect "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1/myawesomelistv1connect"
)

var _ myawesomelistv1connect.AwesomeServiceHandler = (*AwesomeService)(nil)

// AwesomeService implements the Awesome RPC service.
type AwesomeService struct {
	clients *awesome.Awesome
}

// NewAwesomeService constructs an AwesomeService with the given clients.
func NewAwesomeService(clients *awesome.Awesome) *AwesomeService {
	return &AwesomeService{clients: clients}
}

// ListCollections returns collections for the specified repositories.
func (s *AwesomeService) ListCollections(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.ListCollectionsRequest],
) (
	*connect.Response[myawesomelistv1.ListCollectionsResponse],
	error,
) {
	tracer := otel.Tracer("myawesomelist/grpc")
	ctx, span := tracer.Start(ctx, "AwesomeService.ListCollections")
	span.SetAttributes(attribute.Int("repos_len", len(req.Msg.GetRepos())))
	defer span.End()
	repos := req.Msg.GetRepos()
	if len(repos) == 0 {
		for _, rr := range github.DefaultGitHubRepos {
			repos = append(repos, rr.Repo)
		}
	}

	cols, err := s.clients.GitHub().ListCollections(ctx, repos)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(
		&myawesomelistv1.ListCollectionsResponse{Collections: cols},
	), nil
}

// GetCollection returns a single collection for the specified repository.
func (s *AwesomeService) GetCollection(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.GetCollectionRequest],
) (
	*connect.Response[myawesomelistv1.GetCollectionResponse],
	error,
) {
	tracer := otel.Tracer("myawesomelist/grpc")
	ctx, span := tracer.Start(ctx, "AwesomeService.GetCollection")
	defer span.End()
	repo := req.Msg.GetRepo()
	if repo == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeInvalidArgument, errors.New("repo is required")),
		)
	}
	switch repo.GetHostname() {
	case "github.com":
		coll, err := s.clients.GitHub().GetCollection(ctx, repo)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return connect.NewResponse(
			&myawesomelistv1.GetCollectionResponse{Collection: coll},
		), nil
	default:
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeUnimplemented, errors.New("hostname is not supported")),
		)
	}
}

// ListCategories returns categories for the specified repository.
func (s *AwesomeService) ListCategories(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.ListCategoriesRequest],
) (
	*connect.Response[myawesomelistv1.ListCategoriesResponse],
	error,
) {
	tracer := otel.Tracer("myawesomelist/grpc")
	ctx, span := tracer.Start(ctx, "AwesomeService.ListCategories")
	defer span.End()
	repo := req.Msg.GetRepo()
	if repo == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeInvalidArgument, errors.New("repo is required")),
		)
	}
	switch repo.GetHostname() {
	case "github.com":
		coll, err := s.clients.GitHub().GetCollection(ctx, repo)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return connect.NewResponse(
			&myawesomelistv1.ListCategoriesResponse{Categories: coll.Categories},
		), nil
	default:
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeUnimplemented, errors.New("hostname is not supported")),
		)
	}
}

// ListProjects returns projects under the specified category in the repository.
func (s *AwesomeService) ListProjects(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.ListProjectsRequest],
) (
	*connect.Response[myawesomelistv1.ListProjectsResponse],
	error,
) {
	tracer := otel.Tracer("myawesomelist/grpc")
	ctx, span := tracer.Start(ctx, "AwesomeService.ListProjects")
	defer span.End()
	repo := req.Msg.GetRepo()
	if repo == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeInvalidArgument, errors.New("repo is required")),
		)
	}
	switch repo.GetHostname() {
	case "github.com":
		coll, err := s.clients.GitHub().GetCollection(ctx, repo)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		var projects []*myawesomelistv1.Project
		for _, c := range coll.Categories {
			if c.Name == req.Msg.GetCategoryName() {
				projects = c.Projects
				break
			}
		}
		return connect.NewResponse(&myawesomelistv1.ListProjectsResponse{Projects: projects}), nil
	default:
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeUnimplemented, errors.New("hostname is not supported")),
		)
	}
}

func (s *AwesomeService) SearchProjects(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.SearchProjectsRequest],
) (
	*connect.Response[myawesomelistv1.SearchProjectsResponse],
	error,
) {
	tracer := otel.Tracer("myawesomelist/grpc")
	ctx, span := tracer.Start(ctx, "AwesomeService.SearchProjects")
	defer span.End()
	q := req.Msg.GetQuery()
	limit := req.Msg.GetLimit()
	repos := req.Msg.GetRepos()
	slog.DebugContext(
		ctx,
		"search projects request",
		"query",
		q,
		"limit",
		limit,
		"repos",
		len(repos),
	)
	projects, err := s.clients.Agent().SearchProjects(ctx, req.Msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	slog.DebugContext(ctx, "search projects response", "count", len(projects))
	return connect.NewResponse(&myawesomelistv1.SearchProjectsResponse{Projects: projects}), nil
}

// GetProjectStats returns per-repo stats (stars, open issues) persisted in datastore.
func (s *AwesomeService) GetProjectStats(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.GetProjectStatsRequest],
) (
	*connect.Response[myawesomelistv1.GetProjectStatsResponse],
	error,
) {
	tracer := otel.Tracer("myawesomelist/grpc")
	ctx, span := tracer.Start(ctx, "AwesomeService.GetProjectStats")
	defer span.End()
	repo := req.Msg.GetRepo()
	if repo == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeInvalidArgument, errors.New("repo is required")),
		)
	}

	switch repo.GetHostname() {
	case "github.com":
		stats, err := s.clients.GitHub().GetProjectStats(ctx, repo)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return connect.NewResponse(&myawesomelistv1.GetProjectStatsResponse{Stats: stats}), nil
	default:
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeUnimplemented, errors.New("hostname is not supported")),
		)
	}
}
