package main

import (
	"log"
	"net/http"
	"os"

	"myawesomelist.shikanime.studio/app/server"
	"myawesomelist.shikanime.studio/internal/awesome"
	"myawesomelist.shikanime.studio/internal/datastore"
)

func main() {
	// Initialize GitHub client
	client := awesome.NewClientSet()

	// Initialize SQLite datastore
	dbPath := getEnvOrDefault("DATABASE_PATH", "./myawesomelist.db")
	ds, err := datastore.OpenSQLiteDatastore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize datastore: %v", err)
	}
	defer ds.Close()

	// Initialize datastore service
	datastoreService := datastore.NewService(ds, client.GitHub)

	// Initialize server
	srv := server.New(client, datastoreService)

	// Setup routes
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Start server
	port := getEnvOrDefault("PORT", "8080")
	log.Printf("Starting server on port %s", port)
	log.Printf("Database path: %s", dbPath)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
