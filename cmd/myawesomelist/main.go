package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"myawesomelist.shikanime.studio/internal/awesome"
	"myawesomelist.shikanime.studio/internal/awesome/http"
	"myawesomelist.shikanime.studio/internal/config"
	"myawesomelist.shikanime.studio/internal/database"
)

func main() {
	ctx := context.Background()
	cfg := config.New()
	cleanup, err := config.SetupTelemetry(ctx, cfg)
	if err != nil {
		slog.WarnContext(ctx, "telemetry setup failed", "error", err)
	} else {
		defer cleanup()
	}
	config.SetupLog(cfg)
	if err := NewCmdForConfig(cfg).Execute(); err != nil {
		slog.ErrorContext(ctx, "command execution failed", "error", err)
		os.Exit(1)
	}
}

var (
	addr string
	dsn  string
)

// RunServerWithConf runs the HTTP server with the given configuration.
func RunServerWithConf(cfg *config.Config) error {
	ctx := context.Background()
	cfg.Watch(ctx)
	if dsn != "" {
		cfg.Set("DSN", dsn)
	}
	srv, err := http.NewServerForConfig(cfg)
	if err != nil {
		return err
	}
	if addr == "" {
		addr = cfg.GetAddr()
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(addr) }()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil {
			slog.ErrorContext(ctx, "server failed", "error", err)
		}
		if cerr := srv.Close(); cerr != nil {
			slog.ErrorContext(ctx, "shutdown error", "error", cerr)
		}
	case sig := <-quit:
		slog.InfoContext(ctx, "received signal; shutting down", "signal", sig.String())
		if err := srv.Close(); err != nil {
			slog.ErrorContext(ctx, "shutdown error", "error", err)
		}
		slog.InfoContext(ctx, "server stopped")
	}
	return nil
}

// RunMigrateUpWithConf applies all pending migrations with the given configuration.
func RunMigrateUpWithConf(cfg *config.Config) error {
	if dsn != "" {
		cfg.Set("DSN", dsn)
	}
	mg, err := database.NewMigratorForConfig(cfg)
	if err != nil {
		return err
	}
	return mg.Up()
}

// RunMigrateDownWithConf reverts all applied migrations with the given configuration.
func RunMigrateDownWithConf(cfg *config.Config) error {
	if dsn != "" {
		cfg.Set("DSN", dsn)
	}
	mg, err := database.NewMigratorForConfig(cfg)
	if err != nil {
		return err
	}
	return mg.Down()
}

func RunEmbedAllProjectsWithConf(cfg *config.Config) error {
	if dsn != "" {
		cfg.Set("DSN", dsn)
	}
	aw, err := awesome.NewForConfig(cfg)
	if err != nil {
		return err
	}
	defer aw.Close()
	return aw.Agent().UpsertAllStaledProjectEmbeddings(context.Background(), cfg.GetProjectEmbeddingsTTL())
}

// NewServeCmdForConf returns a new cobra.Command for running the API server with the given configuration.
func NewServerStartCmdForConfig(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "start",
		Short: "Run the API server",
		RunE:  func(_ *cobra.Command, _ []string) error { return RunServerWithConf(cfg) },
	}
	c.Flags().
		StringVar(&addr, "addr", "", "Address to run the server on (host:port). If empty, uses HOST and PORT environment variables")
	return c
}

func NewServerCmdForConfig(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{Use: "server", Short: "Server commands"}
	c.AddCommand(NewServerStartCmdForConfig(cfg))
	return c
}

// NewMigrateUpCmdForConf returns a new cobra.Command for applying all pending migrations with the given configuration.
func NewMigrateUpCmdForConfig(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Apply all pending migrations",
		RunE:  func(_ *cobra.Command, _ []string) error { return RunMigrateUpWithConf(cfg) },
	}
}

// NewMigrateDownCmdForConf returns a new cobra.Command for reverting all applied migrations with the given configuration.
func NewMigrateDownCmdForConfig(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "Revert all applied migrations",
		RunE:  func(_ *cobra.Command, _ []string) error { return RunMigrateDownWithConf(cfg) },
	}
}

// NewMigrateCmdForConf returns a new cobra.Command for database migrations with the given configuration.
func NewMigrateCmdForConfig(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{Use: "migrations", Short: "Database migrations"}
	c.AddCommand(NewMigrateUpCmdForConfig(cfg), NewMigrateDownCmdForConfig(cfg))
	return c
}

func NewJobsEmbStartCmdForConfig(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "start",
		Short: "Embed staled project embeddings",
		RunE:  func(_ *cobra.Command, _ []string) error { return RunEmbedAllProjectsWithConf(cfg) },
	}
	return c
}

func NewJobsEmbCmdForConfig(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{Use: "embeding", Short: "Embeddings jobs"}
	c.AddCommand(NewJobsEmbStartCmdForConfig(cfg))
	return c
}

func NewJobsCmdForConfig(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{Use: "jobs", Short: "Background jobs"}
	c.AddCommand(NewJobsEmbCmdForConfig(cfg))
	return c
}

// NewCmdForConf returns a new cobra.Command for the awesome list server and utilities with the given configuration.
func NewCmdForConfig(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{Use: "myawesomelist", Short: "Awesome list server and utilities"}
	c.PersistentFlags().
		StringVar(&dsn, "dsn", "", "Database source name in the format driver://dataSourceName. Falls back to DSN environment variable")
	c.AddCommand(NewServerCmdForConfig(cfg), NewMigrateCmdForConfig(cfg), NewJobsCmdForConfig(cfg))
	return c
}
