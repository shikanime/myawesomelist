package app

import (
	"log"
	"net/http"

	"myawesomelist.shikanime.studio/internal/awesome"
	myawesomelistv1connect "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1/myawesomelistv1connect"
)

type Server struct {
	cs *awesome.ClientSet
}

func New() *Server {
	return &Server{
		cs: awesome.NewClientSet(),
	}
}

func (s *Server) Start(addr string) error {
	http.HandleFunc("/health", s.handleHealth)
	svc := NewAwesomeService(s.cs)
	path, handler := myawesomelistv1connect.NewAwesomeServiceHandler(svc)
	http.Handle(path, handler)
	log.Printf("Server starting on %s", addr)
	log.Printf("gRPC (Connect/gRPC-Web) mounted at %s", path)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"myawesomelist"}`))
}
