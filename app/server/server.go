package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/google/go-github/v75/github"
	"myawesomelist.shikanime.studio/app/templates"
	"myawesomelist.shikanime.studio/internal/awesome"
)

type Server struct {
	client *github.Client
}

var repos = []awesome.GetCollectionsConfig{
	{
		Language: "Go",
		CategoriesConfig: awesome.GetCategoriesConfig{
			ContentConfig: awesome.GetContentConfig{
				Owner: "avelino",
				Repo:  "awesome-go",
			},
			StartSection: "Actor Model",
		},
	},
	{
		Language: "Elixir",
		CategoriesConfig: awesome.GetCategoriesConfig{
			ContentConfig: awesome.GetContentConfig{
				Owner: "h4cc",
				Repo:  "awesome-elixir",
			},
			StartSection: "Actors",
		},
	},
	{
		Language: "JavaScript",
		CategoriesConfig: awesome.GetCategoriesConfig{
			ContentConfig: awesome.GetContentConfig{
				Owner: "sorrycc",
				Repo:  "awesome-javascript",
			},
			StartSection: "Package Managers",
		},
	},
}

func New() *Server {
	return &Server{
		client: github.NewClient(nil),
	}
}

func (s *Server) Start(port string) error {
	http.HandleFunc("/", s.handleHome)
	http.HandleFunc("/health", s.handleHealth)

	log.Printf("Server starting on port %s", port)
	log.Printf("Visit http://localhost:%s to view the application", port)
	return http.ListenAndServe(":"+port, nil)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Printf("Fetching projects from GitHub repositories...")

	collections, err := awesome.GetCollections(ctx, s.client, repos)
	if err != nil {
		log.Printf("Failed to get collections: %v", err)
		http.Error(w, "Failed to load project collections", http.StatusInternalServerError)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Render the template
	component := templates.CollectionsPage(collections)
	err = component.Render(ctx, w)
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
