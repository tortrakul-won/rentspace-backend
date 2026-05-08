package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"rentspace/backend/internal/store"
)

type BookingsHandler struct {
	q *store.Queries
}

func NewBookingsHandler(q *store.Queries) *BookingsHandler {
	return &BookingsHandler{q: q}
}

// Create godoc
// @Summary     Create a booking
// @Tags        bookings
// @Accept      json
// @Produce     json
// @Param       body body     handler.CreateBookingRequest  true  "Booking details"
// @Success     201  {object} handler.BookingResponse
// @Failure     400  {object} handler.ErrorResponse
// @Failure     409  {object} handler.ErrorResponse "Space not available"
// @Failure     500  {object} handler.ErrorResponse
// @Router      /bookings [post]
func (h *BookingsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body store.CreateBookingParams
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	count, err := h.q.CheckOverlappingBookings(r.Context(), store.CheckOverlappingBookingsParams{
		SpaceID:   body.SpaceID,
		StartTime: body.StartTime,
		EndTime:   body.EndTime,
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to check availability")
		return
	}
	if count > 0 {
		Error(w, http.StatusConflict, "space is not available for the requested time")
		return
	}

	booking, err := h.q.CreateBooking(r.Context(), body)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to create booking")
		return
	}
	JSON(w, http.StatusCreated, booking)
}

// Get godoc
// @Summary     Get a booking by ID
// @Tags        bookings
// @Produce     json
// @Param       id  path     string  true  "Booking UUID"
// @Success     200 {object} handler.BookingResponse
// @Failure     400 {object} handler.ErrorResponse
// @Failure     404 {object} handler.ErrorResponse
// @Router      /bookings/{id} [get]
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

// ListBySpace godoc
// @Summary     List bookings for a space
// @Tags        bookings
// @Produce     json
// @Param       id  path     string  true  "Space UUID"
// @Success     200 {array}  handler.BookingResponse
// @Failure     400 {object} handler.ErrorResponse
// @Failure     500 {object} handler.ErrorResponse
// @Router      /spaces/{id}/bookings [get]
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

// UpdateStatus godoc
// @Summary     Update booking status
// @Tags        bookings
// @Accept      json
// @Produce     json
// @Param       id   path     string                      true  "Booking UUID"
// @Param       body body     handler.UpdateStatusRequest true  "New status"
// @Success     200  {object} handler.BookingResponse
// @Failure     400  {object} handler.ErrorResponse
// @Failure     500  {object} handler.ErrorResponse
// @Router      /bookings/{id}/status [patch]
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
