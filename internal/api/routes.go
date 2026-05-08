package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"rentspace/backend/internal/handler"
	appMiddleware "rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

func NewRouter(q store.Querier, jwtSecret string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestSize(1 * 1024 * 1024)) // 1MB max body size
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}))

	// GET /health — server liveness check, no auth required
	r.Get("/health", handler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		authHandler := handler.NewAuthHandler(q, jwtSecret)

		// POST /auth/register — create account + first profile, returns JWT
		r.Post("/auth/register", authHandler.Register)
		// POST /auth/login — verify credentials, returns JWT + all profiles
		r.Post("/auth/login", authHandler.Login)

		// all routes below require a valid JWT in Authorization: Bearer <token>
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.RequireAuth(jwtSecret))

			// GET  /auth/me             — current user, all profiles, active profile ID
			r.Get("/auth/me", authHandler.CurrentUser)
			// POST /auth/switch-profile — swap active profile, returns new JWT
			r.Post("/auth/switch-profile", authHandler.SwitchProfile)
			// POST /auth/profiles       — add a second profile (owner or renter) to the account
			r.Post("/auth/profiles", authHandler.AddProfile)

			spacesHandler := handler.NewSpacesHandler(q)
			// GET    /spaces       — list all active spaces
			r.Get("/spaces", spacesHandler.List)
			// GET    /spaces/{id}  — get a single space by ID
			r.Get("/spaces/{id}", spacesHandler.Get)
			// POST   /spaces       — create a space (owner profile only; owner_id taken from JWT)
			r.Post("/spaces", spacesHandler.Create)
			// PUT    /spaces/{id}  — update a space (owner profile only)
			r.Put("/spaces/{id}", spacesHandler.Update)
			// DELETE /spaces/{id}  — deactivate a space (owner profile only)
			r.Delete("/spaces/{id}", spacesHandler.Deactivate)

			bookingsHandler := handler.NewBookingsHandler(q)
			// POST  /bookings             — create a booking (renter profile only; renter_id taken from JWT)
			r.Post("/bookings", bookingsHandler.Create)
			// GET   /bookings/{id}        — get a single booking by ID
			r.Get("/bookings/{id}", bookingsHandler.Get)
			// PATCH /bookings/{id}/status — update booking status (pending → confirmed → completed / cancelled)
			r.Patch("/bookings/{id}/status", bookingsHandler.UpdateStatus)
			// GET   /spaces/{id}/bookings — list all bookings for a space
			r.Get("/spaces/{id}/bookings", bookingsHandler.ListBySpace)
		})
	})

	return r
}
