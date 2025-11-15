package github

import (
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

type GitHubRepoConfig struct {
	Repo    *myawesomelistv1.Repository
	Options []GetCollectionOption
}

var DefaultGitHubRepos = []GitHubRepoConfig{
	{
		Repo: &myawesomelistv1.Repository{
			Hostname: "github.com",
			Owner:    "avelino",
			Repo:     "awesome-go",
		},
		Options: []GetCollectionOption{WithStartSection("Actor Model"), WithSubsectionAsCategory()},
	},
	{
		Repo: &myawesomelistv1.Repository{
			Hostname: "github.com",
			Owner:    "h4cc",
			Repo:     "awesome-elixir",
		},
		Options: []GetCollectionOption{WithStartSection("Actors")},
	},
	{
		Repo: &myawesomelistv1.Repository{
			Hostname: "github.com",
			Owner:    "sorrycc",
			Repo:     "awesome-javascript",
		},
		Options: []GetCollectionOption{
			WithStartSection("Package Managers"),
			WithEndSection("Worth Reading"),
		},
	},
	{
		Repo: &myawesomelistv1.Repository{
			Hostname: "github.com",
			Owner:    "gostor",
			Repo:     "awesome-go-storage",
		},
		Options: []GetCollectionOption{WithStartSection("Storage Server")},
	},
}
