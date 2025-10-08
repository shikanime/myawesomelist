package app

import (
	"context"
	"log"
	"net/http"

	"myawesomelist.shikanime.studio/internal/awesome"
)

type Server struct {
	client *awesome.ClientSet
}

func New() *Server {
	return &Server{
		client: awesome.NewClientSet(),
	}
}

func (s *Server) Start(addr string) error {
	http.HandleFunc("/", s.handleHome)
	http.HandleFunc("/health", s.handleHealth)
	log.Printf("Server starting on %s", addr)
	log.Printf("Visit http://%s to view the application", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	log.Printf("Fetching projects from GitHub repositories with stargazer counts...")

	var collections []awesome.Collection
	for _, repo := range awesome.DefaultGitHubRepos {
		collection, err := s.client.GitHub.GetCollection(
			ctx,
			repo.Owner,
			repo.Repo,
			repo.Options...,
		)
		if err != nil {
			log.Printf("Failed to get collection for %s/%s: %v", repo.Owner, repo.Repo, err)
			http.Error(w, "Failed to load project collections", http.StatusInternalServerError)
			return
		}
		collections = append(collections, collection)
	}

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Render the template
	component := CollectionsPage(collections)
	err := component.Render(ctx, w)
	if err != nil {
		log.Printf("Failed to render template: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"myawesomelist"}`))
}
