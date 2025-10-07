package awesome

// GitHubRepoConfig represents configuration for a GitHub repository
type GitHubRepoConfig struct {
	Owner   string
	Repo    string
	Options []Option
}

// DefaultGitHubRepos contains the default list of awesome repositories to fetch
var DefaultGitHubRepos = []GitHubRepoConfig{
	{
		Owner: "avelino",
		Repo:  "awesome-go",
		Options: []Option{
			WithStartSection("Actor Model"),
		},
	},
	{
		Owner: "h4cc",
		Repo:  "awesome-elixir",
		Options: []Option{
			WithStartSection("Actors"),
		},
	},
	{
		Owner: "sorrycc",
		Repo:  "awesome-javascript",
		Options: []Option{
			WithStartSection("Package Managers"),
			WithEndSection("Worth Reading"),
		},
	},
	{
		Owner: "gostor",
		Repo:  "awesome-go-storage",
		Options: []Option{
			WithStartSection("Storage Server"),
		},
	},
}
