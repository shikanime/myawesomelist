package http

import (
	"context"
	"fmt"
	"log/slog"
	stdhttp "net/http"

	"connectrpc.com/connect"
	grpchealth "connectrpc.com/grpchealth"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"myawesomelist.shikanime.studio/internal/awesome"
	"myawesomelist.shikanime.studio/internal/awesome/grpc"
	"myawesomelist.shikanime.studio/internal/config"
	myawesomelistv1connect "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1/myawesomelistv1connect"
)

// Server holds handlers and dependencies for the Awesome HTTP server.
type Server struct {
	clients *awesome.Awesome
	mux     *stdhttp.ServeMux
}

// NewServer initializes a Server and mounts the Awesome service and gRPC health handler.
func NewServer(clients *awesome.Awesome) *Server {
	mux := stdhttp.NewServeMux()
	path, handler := myawesomelistv1connect.NewAwesomeServiceHandler(
		grpc.NewAwesomeService(clients),
	)
	mux.Handle(path, handler)
	hpath, hhandler := grpchealth.NewHandler(HealthChecker{clients: clients})
	mux.Handle(hpath, hhandler)
	return &Server{
		clients: clients,
		mux:     mux,
	}
}

// NewServerForConfig builds Awesome clients from cfg and returns a configured Server.
func NewServerForConfig(cfg *config.Config) (*Server, error) {
	clients, err := awesome.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewServer(clients), nil
}

// Close gracefully shuts down the server and closes database connections
func (s *Server) Close() error {
	if s.clients != nil {
		return s.clients.Close()
	}
	return nil
}

// ListenAndServe starts the HTTP server on addr using the internal mux.
func (s *Server) ListenAndServe(addr string) error {
	slog.Info("server starting", "addr", addr)
	return stdhttp.ListenAndServe(addr, otelhttp.NewHandler(s.mux, "http.server"))
}

// HealthChecker reports health based on database connectivity.
type HealthChecker struct{ clients *awesome.Awesome }

// Check implements grpchealth.Checker. It returns StatusServing when the database ping succeeds.
func (c HealthChecker) Check(
	ctx context.Context,
	req *grpchealth.CheckRequest,
) (*grpchealth.CheckResponse, error) {
	tracer := otel.Tracer("myawesomelist/http")
	ctx, span := tracer.Start(ctx, "HealthChecker.Check")
	defer span.End()
	switch req.Service {
	case "":
		return &grpchealth.CheckResponse{Status: grpchealth.StatusNotServing}, nil
	case myawesomelistv1connect.AwesomeServiceName:
		if err := c.clients.Ping(ctx); err != nil {
			return &grpchealth.CheckResponse{Status: grpchealth.StatusNotServing}, nil
		}
		return &grpchealth.CheckResponse{Status: grpchealth.StatusServing}, nil
	default:
		return nil, connect.NewError(
			connect.CodeNotFound,
			fmt.Errorf("unknown service: %s", req.Service),
		)
	}
}
