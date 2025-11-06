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

func NewServer(ds *awesome.DataStore, opts ...awesome.ClientSetOption) *Server {
	return &Server{
		cs: awesome.NewClientSet(ds, opts...),
	}
}

// Close gracefully shuts down the server and closes database connections
func (s *Server) Close() error {
	if s.cs != nil {
		return s.cs.Close()
	}
	return nil
}

func (s *Server) ListenAndServe(addr string) error {
	path, handler := myawesomelistv1connect.NewAwesomeServiceHandler(NewAwesomeService(s.cs))
	http.Handle(path, handler)
	log.Printf("Server starting on %s", addr)
	log.Printf("gRPC (Connect/gRPC-Web) mounted at %s", path)
	return http.ListenAndServe(addr, nil)
}
