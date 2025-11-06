package awesome

import (
	"context"
	"fmt"
)

// ClientSet aggregates external clients used by the application.
type ClientSet struct {
	ds   *DataStore
	opts ClientSetOptions
}

// ClientSetOptions holds configuration for initializing a ClientSet.
type ClientSetOptions struct {
	github []GitHubClientOption
}

// ClientSetOption applies a configuration to ClientSetOptions.
type ClientSetOption func(*ClientSetOptions)

// WithGitHubOptions forwards GitHub client options into the ClientSet configuration.
func WithGitHubOptions(opts ...GitHubClientOption) ClientSetOption {
	return func(o *ClientSetOptions) { o.github = append(o.github, opts...) }
}

// NewClientSet constructs a ClientSet with the given datastore and options.
func NewClientSet(ds *DataStore, opts ...ClientSetOption) *ClientSet {
	var o ClientSetOptions
	for _, opt := range opts {
		opt(&o)
	}
	return &ClientSet{
		ds:   ds,
		opts: o,
	}
}

// GitHub returns the configured GitHub client, or nil if not set.
func (cs *ClientSet) GitHub() *GitHubClient {
	return NewGitHubClient(cs.ds, cs.opts.github...)
}

// Core returns the core datastore-backed API, or nil if not set.
func (cs *ClientSet) Core() *Core {
	return NewCoreClient(cs.ds)
}

func (cs *ClientSet) Close() error {
	if cs.ds != nil {
		return cs.ds.Close()
	}
	return nil
}

// Ping verifies that all configured clients are reachable.
func (cs *ClientSet) Ping(ctx context.Context) error {
	if cs.GitHub() == nil {
		return fmt.Errorf("clients not configured")
	}
	return cs.GitHub().Ping(ctx)
}
