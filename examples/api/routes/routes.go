package routes

import (
	"github.com/hangell/jano"
	"github.com/hangell/jano/examples/api/handlers"
	"log"
	"net/http"
)

func SetupRoutes(app *jano.Jano) {
	app.Use(loggingMiddleware)

	app.Get("/people", handlers.GetPeople)
	app.Post("/people", handlers.CreatePerson)
	app.Get("/people/{id}", handlers.GetPerson)
	app.Put("/people/{id}", handlers.UpdatePerson)
	app.Delete("/people/{id}", handlers.DeletePerson)

	app.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Custom 404: Page not found"))
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
