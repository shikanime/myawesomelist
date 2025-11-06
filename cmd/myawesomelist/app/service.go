package app

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"myawesomelist.shikanime.studio/internal/awesome"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
	myawesomelistv1connect "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1/myawesomelistv1connect"
)

var _ myawesomelistv1connect.AwesomeServiceHandler = (*AwesomeService)(nil)

type AwesomeService struct {
	cs *awesome.ClientSet
}

func NewAwesomeService(clients *awesome.ClientSet) *AwesomeService {
	return &AwesomeService{cs: clients}
}

func (s *AwesomeService) Liveness(
	ctx context.Context,
	_ *connect.Request[myawesomelistv1.LivenessRequest],
) (
	*connect.Response[myawesomelistv1.LivenessResponse],
	error,
) {
	if err := s.cs.Ping(ctx); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&myawesomelistv1.LivenessResponse{}), nil
}

func (s *AwesomeService) Readiness(
	ctx context.Context,
	_ *connect.Request[myawesomelistv1.ReadinessRequest],
) (
	*connect.Response[myawesomelistv1.ReadinessResponse],
	error,
) {
	if err := s.cs.Ping(ctx); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&myawesomelistv1.ReadinessResponse{}), nil
}

func (s *AwesomeService) ListCollections(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.ListCollectionsRequest],
) (
	*connect.Response[myawesomelistv1.ListCollectionsResponse],
	error,
) {
	collections := make([]*myawesomelistv1.Collection, 0, len(awesome.DefaultGitHubRepos))
	for _, rr := range awesome.DefaultGitHubRepos {
		coll, err := s.cs.GitHub().GetCollection(ctx, rr.Owner, rr.Repo)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		// Use protobuf collection directly
		collections = append(collections, coll)
	}
	return connect.NewResponse(&myawesomelistv1.ListCollectionsResponse{Collections: collections}), nil
}

func (s *AwesomeService) GetCollection(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.GetCollectionRequest],
) (
	*connect.Response[myawesomelistv1.GetCollectionResponse],
	error,
) {
	repo := req.Msg.GetRepo()
	if repo == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeInvalidArgument, errors.New("repo is required")),
		)
	}
	coll, err := s.cs.GitHub().GetCollection(
		ctx,
		repo.GetOwner(),
		repo.GetRepo(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(
		&myawesomelistv1.GetCollectionResponse{Collection: coll},
	), nil
}

func (s *AwesomeService) ListCategories(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.ListCategoriesRequest],
) (
	*connect.Response[myawesomelistv1.ListCategoriesResponse],
	error,
) {
	repo := req.Msg.GetRepo()
	if repo == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeInvalidArgument, errors.New("repo is required")),
		)
	}
	coll, err := s.cs.GitHub().GetCollection(
		ctx,
		repo.GetOwner(),
		repo.GetRepo(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// Protobuf categories already in the collection
	return connect.NewResponse(&myawesomelistv1.ListCategoriesResponse{Categories: coll.Categories}), nil
}

func (s *AwesomeService) ListProjects(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.ListProjectsRequest],
) (
	*connect.Response[myawesomelistv1.ListProjectsResponse],
	error,
) {
	repo := req.Msg.GetRepo()
	if repo == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeInvalidArgument, errors.New("repo is required")),
		)
	}
	coll, err := s.cs.GitHub().GetCollection(
		ctx,
		repo.GetOwner(),
		repo.GetRepo(),
	)
	if err != nil {
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
}

func (s *AwesomeService) SearchProjects(
	ctx context.Context,
	req *connect.Request[myawesomelistv1.SearchProjectsRequest],
) (
	*connect.Response[myawesomelistv1.SearchProjectsResponse],
	error,
) {
	repos := req.Msg.GetRepos()
	q := strings.ToLower(req.Msg.GetQuery())
	limit := req.Msg.GetLimit()
	if limit <= 0 {
		limit = 50
	}

	// Build repo list from request or use defaults
	var repoList []myawesomelistv1.Repository
	if len(repos) == 0 {
		for _, rr := range awesome.DefaultGitHubRepos {
			repoList = append(repoList, myawesomelistv1.Repository{Owner: rr.Owner, Repo: rr.Repo})
		}
	} else {
		for _, r := range repos {
			if r == nil {
				continue
			}
			repoList = append(repoList, myawesomelistv1.Repository{Owner: r.GetOwner(), Repo: r.GetRepo()})
		}
	}

	projects, err := s.cs.Core().SearchProjects(ctx, q, limit, repoList)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&myawesomelistv1.SearchProjectsResponse{Projects: projects}), nil
}
