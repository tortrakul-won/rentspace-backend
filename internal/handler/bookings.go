package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

type BookingsHandler struct {
	q store.Querier
}

func NewBookingsHandler(q store.Querier) *BookingsHandler {
	return &BookingsHandler{q: q}
}

// Create pulls renter_id from the JWT — the request body cannot override it.
func (h *BookingsHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "renter" {
		Error(w, http.StatusForbidden, "only renter profiles can create bookings")
		return
	}

	var body CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	spaceID, err := parseUUID(body.SpaceID)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid space_id")
		return
	}
	startTime, err := time.Parse(time.RFC3339, body.StartTime)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid start_time, use RFC3339 format")
		return
	}
	endTime, err := time.Parse(time.RFC3339, body.EndTime)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid end_time, use RFC3339 format")
		return
	}

	count, err := h.q.CheckOverlappingBookings(r.Context(), store.CheckOverlappingBookingsParams{
		SpaceID:   spaceID,
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to check availability")
		return
	}
	if count > 0 {
		Error(w, http.StatusConflict, "space is not available for the requested time")
		return
	}

	booking, err := h.q.CreateBooking(r.Context(), store.CreateBookingParams{
		SpaceID:     spaceID,
		RenterID:    claims.ProfileID,
		StartTime:   startTime,
		EndTime:     endTime,
		TotalPrice:  body.TotalPrice,
		PlatformFee: body.PlatformFee,
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to create booking")
		return
	}
	JSON(w, http.StatusCreated, booking)
}

func (h *BookingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	booking, err := h.q.GetBookingByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "booking not found")
		return
	}
	JSON(w, http.StatusOK, booking)
}

func (h *BookingsHandler) ListBySpace(w http.ResponseWriter, r *http.Request) {
	spaceID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid space id")
		return
	}
	bookings, err := h.q.ListBookingsBySpace(r.Context(), spaceID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to fetch bookings")
		return
	}
	JSON(w, http.StatusOK, bookings)
}

func (h *BookingsHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	booking, err := h.q.UpdateBookingStatus(r.Context(), store.UpdateBookingStatusParams{
		ID:     id,
		Status: body.Status,
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to update booking status")
		return
	}
	JSON(w, http.StatusOK, booking)
}

type UpdateStatusRequest struct {
	Status store.BookingStatus `json:"status"`
}
