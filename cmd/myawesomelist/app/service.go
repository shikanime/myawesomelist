package app

import (
	"context"
	"errors"
	"strings"

	"myawesomelist.shikanime.studio/internal/awesome"

	"connectrpc.com/connect"
	v1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
	myawesomelistv1connect "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1/myawesomelistv1connect"
)

type AwesomeService struct {
	cs *awesome.ClientSet
}

func NewAwesomeService(clients *awesome.ClientSet) *AwesomeService {
	return &AwesomeService{cs: clients}
}

var _ myawesomelistv1connect.AwesomeServiceHandler = (*AwesomeService)(nil)

func (s *AwesomeService) HealthCheck(
	ctx context.Context,
	_ *connect.Request[v1.HealthCheckRequest],
) (
	*connect.Response[v1.HealthCheckResponse],
	error,
) {
	return connect.NewResponse(&v1.HealthCheckResponse{Status: "ok"}), nil
}

func (s *AwesomeService) ListCollections(
	ctx context.Context,
	req *connect.Request[v1.ListCollectionsRequest],
) (
	*connect.Response[v1.ListCollectionsResponse],
	error,
) {
	repos := req.Msg.GetRepos()
	includeRepo := req.Msg.GetIncludeRepoInfo()

	// Use default repos when none provided
	if len(repos) == 0 {
		collections := make([]*v1.Collection, 0, len(awesome.DefaultGitHubRepos))
		for _, rr := range awesome.DefaultGitHubRepos {
			coll, err := s.cs.GitHub.GetCollection(
				ctx,
				rr.Owner,
				rr.Repo,
				optionsFromInclude(includeRepo)...,
			)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			collections = append(collections, toProtoCollection(coll))
		}
		return connect.NewResponse(
			&v1.ListCollectionsResponse{Collections: collections},
		), nil
	}

	// Use provided repos
	collections := make([]*v1.Collection, 0, len(repos))
	for _, r := range repos {
		if r == nil {
			continue
		}
		coll, err := s.cs.GitHub.GetCollection(
			ctx,
			r.GetOwner(),
			r.GetRepo(),
			optionsFromInclude(includeRepo)...,
		)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		collections = append(collections, toProtoCollection(coll))
	}
	return connect.NewResponse(
		&v1.ListCollectionsResponse{Collections: collections},
	), nil
}

func (s *AwesomeService) GetCollection(
	ctx context.Context,
	req *connect.Request[v1.GetCollectionRequest],
) (
	*connect.Response[v1.GetCollectionResponse],
	error,
) {
	repo := req.Msg.GetRepo()
	if repo == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeInvalidArgument, errors.New("repo is required")),
		)
	}
	coll, err := s.cs.GitHub.GetCollection(
		ctx,
		repo.GetOwner(),
		repo.GetRepo(),
		optionsFromInclude(req.Msg.GetIncludeRepoInfo())...,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(
		&v1.GetCollectionResponse{Collection: toProtoCollection(coll)},
	), nil
}

func (s *AwesomeService) ListCategories(
	ctx context.Context,
	req *connect.Request[v1.ListCategoriesRequest],
) (
	*connect.Response[v1.ListCategoriesResponse],
	error,
) {
	repo := req.Msg.GetRepo()
	if repo == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeInvalidArgument, errors.New("repo is required")),
		)
	}
	coll, err := s.cs.GitHub.GetCollection(
		ctx,
		repo.GetOwner(),
		repo.GetRepo(),
		optionsFromInclude(req.Msg.GetIncludeRepoInfo())...,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cats := make([]*v1.Category, 0, len(coll.Categories))
	for _, c := range coll.Categories {
		cats = append(cats, toProtoCategory(c))
	}
	return connect.NewResponse(
		&v1.ListCategoriesResponse{Categories: cats},
	), nil
}

func (s *AwesomeService) ListProjects(
	ctx context.Context,
	req *connect.Request[v1.ListProjectsRequest],
) (
	*connect.Response[v1.ListProjectsResponse],
	error,
) {
	repo := req.Msg.GetRepo()
	if repo == nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			connect.NewError(connect.CodeInvalidArgument, errors.New("repo is required")),
		)
	}
	coll, err := s.cs.GitHub.GetCollection(
		ctx,
		repo.GetOwner(),
		repo.GetRepo(),
		optionsFromInclude(req.Msg.GetIncludeRepoInfo())...,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var projects []*v1.Project
	for _, c := range coll.Categories {
		if c.Name == req.Msg.GetCategoryName() {
			for _, p := range c.Projects {
				projects = append(projects, toProtoProject(p))
			}
			break
		}
	}
	return connect.NewResponse(
		&v1.ListProjectsResponse{Projects: projects},
	), nil
}

func (s *AwesomeService) SearchProjects(
	ctx context.Context,
	req *connect.Request[v1.SearchProjectsRequest],
) (
	*connect.Response[v1.SearchProjectsResponse],
	error,
) {
	repos := req.Msg.GetRepos()
	includeRepo := req.Msg.GetIncludeRepoInfo()
	q := strings.ToLower(req.Msg.GetQuery())
	limit := req.Msg.GetLimit()
	if limit <= 0 {
		limit = 50
	}

	// Fetch collections from provided repos or defaults
	var repoList []struct{ owner, repo string }
	if len(repos) == 0 {
		for _, rr := range awesome.DefaultGitHubRepos {
			repoList = append(
				repoList,
				struct{ owner, repo string }{rr.Owner, rr.Repo},
			)
		}
	} else {
		for _, r := range repos {
			if r == nil {
				continue
			}
			repoList = append(
				repoList,
				struct{ owner, repo string }{r.GetOwner(), r.GetRepo()},
			)
		}
	}

	var results []*v1.Project
	for _, rr := range repoList {
		coll, err := s.cs.GitHub.GetCollection(
			ctx,
			rr.owner,
			rr.repo,
			optionsFromInclude(includeRepo)...,
		)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, c := range coll.Categories {
			for _, p := range c.Projects {
				if matchesQuery(p, q) {
					results = append(results, toProtoProject(p))
					if int32(len(results)) >= limit {
						return connect.NewResponse(
							&v1.SearchProjectsResponse{Projects: results},
						), nil
					}
				}
			}
		}
	}
	return connect.NewResponse(
		&v1.SearchProjectsResponse{Projects: results},
	), nil
}

// Helpers

func optionsFromInclude(include bool) []awesome.Option {
	if include {
		// Note: WithRepoInfo currently sets includeRepoInfo=false in internal/awesome.
		// We still pass it for future fix; enrichment is skipped until corrected.
		return []awesome.Option{awesome.WithRepoInfo()}
	}
	return nil
}

func toProtoCollection(in awesome.Collection) *v1.Collection {
	out := &v1.Collection{Language: in.Language}
	out.Categories = make([]*v1.Category, 0, len(in.Categories))
	for _, c := range in.Categories {
		out.Categories = append(out.Categories, toProtoCategory(c))
	}
	return out
}

func toProtoCategory(in awesome.Category) *v1.Category {
	out := &v1.Category{Name: in.Name}
	out.Projects = make([]*v1.Project, 0, len(in.Projects))
	for _, p := range in.Projects {
		out.Projects = append(out.Projects, toProtoProject(p))
	}
	return out
}

func toProtoProject(in awesome.Project) *v1.Project {
	out := &v1.Project{
		Name:        in.Name,
		Description: in.Description,
		Url:         in.URL,
	}
	if in.StargazersCount != nil {
		v := int64(*in.StargazersCount)
		out.StargazersCount = &v
	}
	if in.OpenIssueCount != nil {
		v := int64(*in.OpenIssueCount)
		out.OpenIssueCount = &v
	}
	return out
}

func matchesQuery(p awesome.Project, q string) bool {
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(p.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(p.Description), q) {
		return true
	}
	if strings.Contains(strings.ToLower(p.URL), q) {
		return true
	}
	return false
}
