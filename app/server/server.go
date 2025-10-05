package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/google/go-github/v75/github"
	"myawesomelist.shikanime.studio/app/avelinoawesomego"
	"myawesomelist.shikanime.studio/app/h4ccawesomeelixir"
	"myawesomelist.shikanime.studio/app/sorryccawesomejavascript"
	"myawesomelist.shikanime.studio/app/templates"
	"myawesomelist.shikanime.studio/app/types"
)

type Server struct {
	client *github.Client
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

	// Get projects from awesome-go
	goProjects, err := avelinoawesomego.GetProjects(ctx, s.client)
	if err != nil {
		log.Printf("Failed to parse Go projects: %v", err)
		http.Error(w, "Failed to load Go projects", http.StatusInternalServerError)
		return
	}
	log.Printf("Loaded %d Go projects", len(goProjects))

	// Get projects from awesome-elixir
	elixirProjects, err := h4ccawesomeelixir.GetProjects(ctx, s.client)
	if err != nil {
		log.Printf("Failed to parse Elixir projects: %v", err)
		http.Error(w, "Failed to load Elixir projects", http.StatusInternalServerError)
		return
	}
	log.Printf("Loaded %d Elixir projects", len(elixirProjects))

	// Get projects from awesome-javascript
	jsProjects, err := sorryccawesomejavascript.GetProjects(ctx, s.client)
	if err != nil {
		log.Printf("Failed to parse JavaScript projects: %v", err)
		http.Error(w, "Failed to load JavaScript projects", http.StatusInternalServerError)
		return
	}
	log.Printf("Loaded %d JavaScript projects", len(jsProjects))

	// Convert to common types
	collections := []types.ProjectCollection{
		{
			Language: "Go",
			Projects: convertGoProjects(goProjects),
		},
		{
			Language: "Elixir",
			Projects: convertElixirProjects(elixirProjects),
		},
		{
			Language: "JavaScript",
			Projects: convertJavaScriptProjects(jsProjects),
		},
	}

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Render the template
	component := templates.ProjectsPage(collections)
	err = component.Render(ctx, w)
	if err != nil {
		log.Printf("Failed to render template: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully rendered page with %d total projects", len(goProjects)+len(elixirProjects)+len(jsProjects))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"myawesomelist"}`))
}

func convertGoProjects(projects []avelinoawesomego.Project) []types.Project {
	result := make([]types.Project, len(projects))
	for i, p := range projects {
		result[i] = types.Project{
			Name:        p.Name,
			Description: p.Description,
			URL:         p.URL,
			Category:    p.Category,
			Language:    "Go",
		}
	}
	return result
}

func convertElixirProjects(projects []h4ccawesomeelixir.Project) []types.Project {
	result := make([]types.Project, len(projects))
	for i, p := range projects {
		result[i] = types.Project{
			Name:        p.Name,
			Description: p.Description,
			URL:         p.URL,
			Category:    p.Category,
			Language:    "Elixir",
		}
	}
	return result
}

func convertJavaScriptProjects(projects []sorryccawesomejavascript.Project) []types.Project {
	result := make([]types.Project, len(projects))
	for i, p := range projects {
		result[i] = types.Project{
			Name:        p.Name,
			Description: p.Description,
			URL:         p.URL,
			Category:    p.Category,
			Language:    "JavaScript",
		}
	}
	return result
}
