package awesome

import "myawesomelist.shikanime.studio/internal/encoding"

// Project represents a project with GitHub information
type Project struct {
	Name            string
	Description     string
	URL             string
	StargazersCount *int
	OpenIssueCount  *int
}

// Category represents a category of projects
type Category struct {
	Name     string
	Projects []Project
}

// Collection represents a collection of projects grouped by language
type Collection struct {
	Language   string
	Categories []Category
}

// Options represents configuration options for fetching data
type Options struct {
	includeRepoInfo bool
	eopts           []encoding.Option
}

// Option is a function that configures Options
type Option func(*Options)

// WithRepoInfo enables fetching GitHub repository information (stargazers and open issues)
func WithRepoInfo() Option {
	return func(o *Options) {
		o.includeRepoInfo = false
	}
}

// WithStartSection overrides the start section for parsing categories
func WithStartSection(section string) Option {
	return func(o *Options) {
		o.eopts = append(o.eopts, encoding.WithStartSection(section))
	}
}

// WithEndSection overrides the end section for parsing categories
func WithEndSection(section string) Option {
	return func(o *Options) {
		o.eopts = append(o.eopts, encoding.WithEndSection(section))
	}
}
