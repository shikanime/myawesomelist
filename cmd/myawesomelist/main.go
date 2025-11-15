package main

import (
	"errors"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"myawesomelist.shikanime.studio/cmd/myawesomelist/app"
	"myawesomelist.shikanime.studio/internal/awesome"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var (
	rootCmd = &cobra.Command{
		Use:   "myawesomelist",
		Short: "Awesome list server and utilities",
	}
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Run the API server",
		RunE:  runServer,
	}
	migrateCmd = &cobra.Command{
		Use:   "migrate",
		Short: "Database migrations",
	}
	upCmd = &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE:  runMigrateUp,
	}
	downCmd = &cobra.Command{
		Use:   "down",
		Short: "Revert all applied migrations",
		RunE:  runMigrateDown,
	}

	// Flags
	addr string
	dsn  string
)

func init() {
	serveCmd.Flags().
		StringVar(&addr, "addr", "", "Address to run the server on (host:port). If empty, uses HOST and PORT environment variables")
	rootCmd.PersistentFlags().
		StringVar(&dsn, "dsn", "", "Database source name in the format driver://dataSourceName. Falls back to DSN environment variable")
	migrateCmd.AddCommand(upCmd, downCmd)
	rootCmd.AddCommand(serveCmd, migrateCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	ai, err := newOpenAIAPIClientFromEnv()
	if err != nil {
		return err
	}

	ds, err := openDataStoreFromEnv(ai)
	if err != nil {
		return err
	}

	srv, err := newServerFromEnv(ds, ai)
	if err != nil {
		return err
	}

	// Fallback to env if flag is empty
	if addr == "" {
		addr = awesome.GetAddr()
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(addr)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			log.Printf("Server failed: %v", err)
		}
		if cerr := srv.Close(); cerr != nil {
			log.Printf("Error during shutdown: %v", cerr)
		}
	case sig := <-quit:
		log.Printf("Received signal %s. Shutting down server...", sig)
		if err := srv.Close(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		log.Println("Server stopped")
	}
	return nil
}

func runMigrateUp(cmd *cobra.Command, args []string) error {
	db, err := openGormDbFromEnv()
	if err != nil {
		return err
	}

	mg, err := app.NewMigrator(db)
	if err != nil {
		return err
	}

	return mg.Up()
}

func runMigrateDown(cmd *cobra.Command, args []string) error {
	db, err := openGormDbFromEnv()
	if err != nil {
		return err
	}

	mg, err := app.NewMigrator(db)
	if err != nil {
		return err
	}

	return mg.Down()
}

// newClientSetFromEnv creates a ClientSet with options from environment variables.
func newClientSetFromEnv(ds *awesome.DataStore) *awesome.ClientSet {
	var opts []awesome.ClientSetOption
	if token := awesome.GetGitHubToken(); token != "" {
		opts = append(
			opts,
			awesome.WithGitHubOptions(
				awesome.WithToken(token),
				awesome.WithLimiter(awesome.NewGitHubLimiter(true)),
			),
		)
	}
	return awesome.NewClientSet(ds, opts...)
}

// newServerFromEnv creates a new Server instance using a ClientSet built from env.
func newServerFromEnv(ds *awesome.DataStore, ai *openai.Client) (*app.Server, error) {
	cs := newClientSetFromEnv(ds)
	return app.NewServer(cs, ai), nil
}

// openDataStoreFromEnv opens a connection to the database and constructs a DataStore.
func openDataStoreFromEnv(ai *openai.Client) (*awesome.DataStore, error) {
	db, err := openGormDbFromEnv()
	if err != nil {
		return nil, err
	}
	return awesome.NewDataStore(
		db,
		awesome.NewOpenAIEmbeddings(ai, awesome.GetEmbeddingModel()),
	), nil
}

// openDbFromEnv opens a connection to the database using the provided DSN or falls back to the environment variable.
func openGormDbFromEnv() (*gorm.DB, error) {
	var dsnUrl *url.URL
	var err error
	if dsn == "" {
		dsnUrl, err = awesome.GetDsn()
		if err != nil {
			return nil, err
		}
	} else {
		dsnUrl, err = url.Parse(dsn)
		if err != nil {
			return nil, err
		}
	}
	if dsnUrl.Scheme == "" {
		return nil, errors.New("invalid DSN: must be in format driver://dataSourceName")
	}

	switch dsnUrl.Scheme {
	case "postgres", "postgresql":
		return gorm.Open(postgres.Open(dsnUrl.String()), &gorm.Config{})
	case "sqlite", "sqlite3":
		return gorm.Open(sqlite.Open(dsnUrl.String()), &gorm.Config{})
	default:
		return nil, errors.New("unsupported driver for gorm: " + dsnUrl.Scheme)
	}
}

func newOpenAIAPIClientFromEnv() (*openai.Client, error) {
	c := openai.NewClient(
		option.WithAPIKey(awesome.GetOpenAIAPIKey()),
	)
	return &c, nil
}
