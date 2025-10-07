package main

import (
	"flag"
	"log"
	"os"

	"myawesomelist.shikanime.studio/app/server"
)

func main() {
	var addr string
	flag.StringVar(&addr, "addr", "", "Address to run the server on (host:port). If empty, uses HOST and PORT environment variables")
	flag.Parse()

	// If addr is not provided via flag, check for legacy port flag or environment
	if addr == "" {
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

		addr = host + ":" + port
	}

	srv := server.New()
	log.Fatal(srv.Start(addr))
}
