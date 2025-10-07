package main

import (
	"flag"
	"log"
	"os"

	"myawesomelist.shikanime.studio/app/server"
)

func main() {
	var port string
	flag.StringVar(&port, "port", "8080", "Port to run the server on")
	flag.Parse()

	// Allow port to be set via environment variable
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	srv := server.New()
	log.Fatal(srv.Start(port))
}
