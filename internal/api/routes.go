package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"

	"rentspace/backend/internal/handler"
	"rentspace/backend/internal/store"
)

func NewRouter(q *store.Queries) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}))

	r.Get("/health", handler.Health)
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Route("/api/v1", func(r chi.Router) {
		spacesHandler := handler.NewSpacesHandler(q)
		r.Get("/spaces", spacesHandler.List)
		r.Get("/spaces/{id}", spacesHandler.Get)
		r.Post("/spaces", spacesHandler.Create)          // TODO: require owner auth
		r.Put("/spaces/{id}", spacesHandler.Update)      // TODO: require owner auth
		r.Delete("/spaces/{id}", spacesHandler.Deactivate) // TODO: require owner auth

		bookingsHandler := handler.NewBookingsHandler(q)
		r.Post("/bookings", bookingsHandler.Create)                  // TODO: require renter auth
		r.Get("/bookings/{id}", bookingsHandler.Get)                 // TODO: require auth
		r.Patch("/bookings/{id}/status", bookingsHandler.UpdateStatus) // TODO: require owner auth
		r.Get("/spaces/{id}/bookings", bookingsHandler.ListBySpace)  // TODO: require owner auth
	})

	return r
}
