package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"

	"rentspace/backend/internal/handler"
	appMiddleware "rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

func NewRouter(q *store.Queries, jwtSecret string) http.Handler {
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
		authHandler := handler.NewAuthHandler(q, jwtSecret)
		r.Post("/auth/register", authHandler.Register)
		r.Post("/auth/login", authHandler.Login)

		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.RequireAuth(jwtSecret))

			r.Get("/auth/me", authHandler.CurrentUser)
			r.Post("/auth/switch-profile", authHandler.SwitchProfile)
			r.Post("/auth/profiles", authHandler.AddProfile)

			spacesHandler := handler.NewSpacesHandler(q)
			r.Get("/spaces", spacesHandler.List)
			r.Get("/spaces/{id}", spacesHandler.Get)
			r.Post("/spaces", spacesHandler.Create)
			r.Put("/spaces/{id}", spacesHandler.Update)
			r.Delete("/spaces/{id}", spacesHandler.Deactivate)

			bookingsHandler := handler.NewBookingsHandler(q)
			r.Post("/bookings", bookingsHandler.Create)
			r.Get("/bookings/{id}", bookingsHandler.Get)
			r.Patch("/bookings/{id}/status", bookingsHandler.UpdateStatus)
			r.Get("/spaces/{id}/bookings", bookingsHandler.ListBySpace)
		})
	})

	return r
}
