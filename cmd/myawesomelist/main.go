package main

import (
	"database/sql"
	"errors"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"myawesomelist.shikanime.studio/cmd/myawesomelist/app"
	"myawesomelist.shikanime.studio/internal/awesome"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
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
	ds, err := openDataStoreFromEnv()
	if err != nil {
		return err
	}

	srv, err := newServerFromEnv(ds)
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
	db, err := openDbFromEnv()
	if err != nil {
		return err
	}
	defer db.Close()

	mg, err := app.NewMigrator(db)
	if err != nil {
		return err
	}
	defer mg.Close()

	return mg.Up()
}

func runMigrateDown(cmd *cobra.Command, args []string) error {
	db, err := openDbFromEnv()
	if err != nil {
		return err
	}
	defer db.Close()

	mg, err := app.NewMigrator(db)
	if err != nil {
		return err
	}
	defer mg.Close()

	return mg.Down()
}

// newServerFromEnv creates a new Server instance with options from environment variables.
func newServerFromEnv(ds *awesome.DataStore) (*app.Server, error) {
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
	return app.NewServer(ds, opts...), nil
}

// openDataStoreFromEnv opens a connection to the database and constructs a DataStore.
func openDataStoreFromEnv() (*awesome.DataStore, error) {
	db, err := openDbFromEnv()
	if err != nil {
		return nil, err
	}
	return awesome.NewDataStore(db), nil
}

// openDbFromEnv opens a connection to the database using the provided DSN or falls back to the environment variable.
func openDbFromEnv() (*sql.DB, error) {
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

	db, err := sql.Open(dsnUrl.Scheme, dsnUrl.String())
	if err != nil {
		return nil, err
	}
	return db, nil
}
