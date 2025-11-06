package awesome

import (
	"myawesomelist.shikanime.studio/internal/encoding"
)

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
		o.includeRepoInfo = true
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

// WithSubsectionAsCategory enables H3 subsections to be treated as categories.
func WithSubsectionAsCategory() Option {
	return func(o *Options) {
		o.eopts = append(o.eopts, encoding.WithSubsectionAsCategory())
	}
}
