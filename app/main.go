package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/a-h/templ"
)

func main() {
	http.Handle("/", templ.Handler(hello("World")))

	port := ":8080"
	fmt.Printf("Server starting on http://localhost%s\n", port)

	log.Fatal(http.ListenAndServe(port, nil))
}
