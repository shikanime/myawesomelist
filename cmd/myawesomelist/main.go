package main

import (
	"database/sql"
	"errors"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
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
	serverCmd = &cobra.Command{
		Use:   "server",
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
	serverCmd.Flags().StringVar(&addr, "addr", "", "Address to run the server on (host:port). If empty, uses HOST and PORT environment variables")
	rootCmd.PersistentFlags().StringVar(&dsn, "dsn", "", "Database source name in the format driver://dataSourceName. Falls back to DSN environment variable")
	migrateCmd.AddCommand(upCmd, downCmd)
	rootCmd.AddCommand(serverCmd, migrateCmd)
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

	// Fallback to env if addr not provided
	finalAddr := addr
	if finalAddr == "" {
		// Check for legacy PORT environment variable or default
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}

		// Check for HOST environment variable or default
		host := os.Getenv("HOST")
		if host == "" {
			host = "localhost"
		}

		finalAddr = host + ":" + port
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(finalAddr)
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
	db, err := openDb()
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
	db, err := openDb()
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

func newServerFromEnv(ds *awesome.DataStore) (*app.Server, error) {
	var opts []awesome.ClientSetOption
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		opts = append(opts, awesome.WithGitHubOptions(awesome.WithToken(token)))
	}
	return app.NewServer(ds, opts...), nil
}

// openDataStoreFromEnv opens a connection to the database and constructs a DataStore.
func openDataStoreFromEnv() (*awesome.DataStore, error) {
	db, err := openDb()
	if err != nil {
		return nil, err
	}
	ds := awesome.NewDataStore(db)
	if err := ds.Connect(); err != nil {
		return nil, err
	}
	return ds, nil
}

func openDb() (*sql.DB, error) {
	var source *url.URL
	var err error
	if dsn != "" {
		source, err = url.Parse(dsn)
		if err != nil {
			return nil, err
		}
	} else {
		source, err = awesome.GetDsn()
		if err != nil {
			return nil, err
		}
	}
	if source.Scheme == "" {
		return nil, errors.New("invalid DSN: must be in format driver://dataSourceName")
	}

	db, err := sql.Open(source.Scheme, source.String())
	if err != nil {
		return nil, err
	}
	return db, nil
}
