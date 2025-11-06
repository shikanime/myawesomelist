package awesome

import (
	"context"
	"fmt"
)

type ClientSet struct {
	GitHub *GitHubClient
}

type ClientSetOptions struct {
	github []GitHubClientOption
}

type ClientSetOption func(*ClientSetOptions)

func WithGitHubOptions(opts ...GitHubClientOption) ClientSetOption {
	return func(o *ClientSetOptions) { o.github = append(o.github, opts...) }
}

func NewClientSet(ds *DataStore, opts ...ClientSetOption) *ClientSet {
	var o ClientSetOptions
	for _, opt := range opts {
		opt(&o)
	}
	return &ClientSet{
		GitHub: NewGitHubClient(ds, o.github...),
	}
}

func (cs *ClientSet) Close() error {
	if cs.GitHub != nil {
		return cs.GitHub.Close()
	}
	return nil
}

func (cs *ClientSet) Ping(ctx context.Context) error {
	if cs.GitHub == nil {
		return fmt.Errorf("clients not configured")
	}
	return cs.GitHub.Ping(ctx)
}
