package types

type Project struct {
	Name        string
	Description string
	URL         string
	Category    string
	Language    string
}

type ProjectCollection struct {
	Language string
	Projects []Project
}
