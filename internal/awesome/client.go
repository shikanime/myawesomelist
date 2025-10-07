package awesome

type ClientSet struct {
	GitHub *GitHubClient
}

func NewClientSet() *ClientSet {
	return &ClientSet{
		GitHub: NewGitHubClient(),
	}
}
