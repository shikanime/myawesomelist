package app

import (
	"log"
	"net/http"

	"github.com/openai/openai-go/v3"
	"myawesomelist.shikanime.studio/internal/awesome"
	myawesomelistv1connect "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1/myawesomelistv1connect"
)

type Server struct {
	cs *awesome.ClientSet
	ai *openai.Client
}

func NewServer(cs *awesome.ClientSet, ai *openai.Client) *Server {
	return &Server{
		cs: cs,
		ai: ai,
	}
}

// Close gracefully shuts down the server and closes database connections
func (s *Server) Close() error {
	if s.cs != nil {
		return s.cs.Close()
	}
	return nil
}

// Livez handles the liveness probe at /livez
func (s *Server) Livez(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.cs.Ping(r.Context()); err != nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("ok"))
	if err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// Readyz handles the readiness probe at /readyz
func (s *Server) Readyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.cs.Ping(r.Context()); err != nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("ok"))
	if err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

func (s *Server) ListenAndServe(addr string) error {
	path, handler := myawesomelistv1connect.NewAwesomeServiceHandler(NewAwesomeService(s.cs))
	http.Handle(path, handler)

	// Register HTTP probe endpoints
	http.HandleFunc("/livez", s.Livez)
	http.HandleFunc("/readyz", s.Readyz)

	log.Printf("Server starting on %s", addr)
	log.Printf("gRPC (Connect/gRPC-Web) mounted at %s", path)
	return http.ListenAndServe(addr, nil)
}
