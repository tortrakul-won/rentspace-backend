package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"rentspace/backend/internal/handler"
	"rentspace/backend/internal/hub"
	appMiddleware "rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

func NewRouter(q store.Store, jwtSecret string, corsOrigins []string, gotenbergURL string) http.Handler {
	h := hub.New()
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestSize(1 * 1024 * 1024)) // 1MB max body size
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: corsOrigins,
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

		// GET /config/payment — public payment config (promptpay details)
		configHandler := handler.NewConfigHandler(q)
		r.Get("/config/payment", configHandler.PaymentConfig)

		// all routes below require a valid JWT in Authorization: Bearer <token>
		spacesHandler := handler.NewSpacesHandler(q)
		// Public reads — OptionalAuth so authenticated users get personalised results (own spaces excluded).
		r.With(appMiddleware.OptionalAuth(jwtSecret)).Get("/spaces", spacesHandler.List)
		r.Get("/spaces/{id}", spacesHandler.Get)
		r.Get("/spaces/{id}/availability", spacesHandler.GetAvailability)

		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.RequireAuth(jwtSecret))

			// GET  /auth/me             — current user, all profiles, active profile ID
			r.Get("/auth/me", authHandler.CurrentUser)
			// POST /auth/switch-profile — swap active profile, returns new JWT
			r.Post("/auth/switch-profile", authHandler.SwitchProfile)
			// POST  /auth/profiles       — add a second profile (owner or renter) to the account
			r.Post("/auth/profiles", authHandler.AddProfile)
			// PATCH /auth/profile — update profile_name and/or line_id for the active profile
			r.Patch("/auth/profile", authHandler.UpdateProfile)

			// GET    /spaces/mine  — list all spaces owned by the authenticated owner profile (active + inactive)
			r.Get("/spaces/mine", spacesHandler.Mine)
			// POST   /spaces       — create a space (owner profile only; owner_id taken from JWT)
			r.Post("/spaces", spacesHandler.Create)
			// PUT    /spaces/{id}  — update a space (owner profile only)
			r.Put("/spaces/{id}", spacesHandler.Update)
			// DELETE /spaces/{id}             — deactivate a space (owner profile only)
			r.Delete("/spaces/{id}", spacesHandler.Deactivate)
			// POST   /spaces/{id}/reactivate  — reactivate a deactivated space (owner only)
			r.Post("/spaces/{id}/reactivate", spacesHandler.Reactivate)
			// DELETE /spaces/{id}/permanent   — permanently delete a space (owner only)
			r.Delete("/spaces/{id}/permanent", spacesHandler.Delete)
			// PUT    /spaces/{id}/availability — replace weekly schedule (owner only)
			r.Put("/spaces/{id}/availability", spacesHandler.SetAvailability)

			bookingsHandler := handler.NewBookingsHandler(q, h)
			// POST  /bookings             — create a booking (renter profile only; renter_id taken from JWT)
			r.Post("/bookings", bookingsHandler.Create)
			// GET   /bookings/mine        — list all bookings for the authenticated renter
			r.Get("/bookings/mine", bookingsHandler.ListMine)
			// GET   /bookings/owner       — list all bookings across owner's spaces
			r.Get("/bookings/owner", bookingsHandler.ListMineOwner)
			// GET   /bookings/owner/{id}  — enriched booking detail for owner review page
			r.Get("/bookings/owner/{id}", bookingsHandler.GetOwnerBookingDetail)
			// GET   /bookings/{id}        — get a single booking by ID
			r.Get("/bookings/{id}", bookingsHandler.Get)
			// PATCH /bookings/{id}/status — update booking status (pending → confirmed / cancelled)
			r.Patch("/bookings/{id}/status", bookingsHandler.UpdateStatus)
			// GET   /spaces/{id}/bookings — list all bookings for a space
			r.Get("/spaces/{id}/bookings", bookingsHandler.ListBySpace)

			documentsHandler := handler.NewDocumentsHandler(q, gotenbergURL)
			// GET /bookings/{id}/documents           — list available documents for a booking
			r.Get("/bookings/{id}/documents", documentsHandler.GetBookingDocumentList)
			// GET /bookings/{id}/documents/{docType} — download a specific document as PDF
			r.Get("/bookings/{id}/documents/{docType}", documentsHandler.GetDocument)

			notificationsHandler := handler.NewNotificationsHandler(q, h)
			// GET   /notifications/stream  — SSE stream of new notifications for the active profile
			r.Get("/notifications/stream", notificationsHandler.Stream)
			// GET   /notifications        — list recent notifications for the active profile
			r.Get("/notifications", notificationsHandler.List)
			// GET   /notifications/unread-count — count unread notifications
			r.Get("/notifications/unread-count", notificationsHandler.UnreadCount)
			// POST  /notifications/{id}/read — mark a notification read
			r.Post("/notifications/{id}/read", notificationsHandler.MarkRead)
			// POST  /notifications/read-all — mark all notifications read
			r.Post("/notifications/read-all", notificationsHandler.MarkAllRead)

			// Admin routes — require is_admin flag in JWT
			r.Group(func(r chi.Router) {
				r.Use(appMiddleware.RequireAdmin)
				adminHandler := handler.NewAdminHandler(q, h)
				// GET  /admin/bookings          — list all payment_pending bookings
				r.Get("/admin/bookings", adminHandler.ListPaymentPending)
				// GET  /admin/bookings/{id}     — full booking detail for admin review page
				r.Get("/admin/bookings/{id}", adminHandler.GetBookingDetail)
				// POST /admin/bookings/{id}/approve         — confirm payment → booking confirmed
				r.Post("/admin/bookings/{id}/approve", adminHandler.Approve)
				// POST /admin/bookings/{id}/reject-retry    — reject slip, renter retries → awaiting_payment
				r.Post("/admin/bookings/{id}/reject-retry", adminHandler.RejectRetry)
				// POST /admin/bookings/{id}/reject-permanent — reject slip permanently → cancelled
				r.Post("/admin/bookings/{id}/reject-permanent", adminHandler.RejectPermanent)
			})
		})
	})

	return r
}
