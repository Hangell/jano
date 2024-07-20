package main

import (
	"github.com/hangell/jano"
	"github.com/hangell/jano/examples/api/routes"
	"log"
	"net/http"
	"os"
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
