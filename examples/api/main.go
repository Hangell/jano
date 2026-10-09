package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Hangell/jano"
	"github.com/Hangell/jano/examples/api/routes"
)

func main() {
	app := jano.New()

	routes.SetupRoutes(app)

	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}
	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, app.Router()))
}
