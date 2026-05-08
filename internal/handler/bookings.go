package handler

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

var errTimeSlotTaken = errors.New("time slot taken")

type BookingsHandler struct {
	q store.Store
}

func NewBookingsHandler(q store.Store) *BookingsHandler {
	return &BookingsHandler{q: q}
}

// Create pulls renter_id from the JWT and calculates price server-side from space rates.
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
	if !startTime.After(time.Now()) {
		Error(w, http.StatusBadRequest, "start_time must be in the future")
		return
	}
	if !endTime.After(startTime) {
		Error(w, http.StatusBadRequest, "end_time must be after start_time")
		return
	}

	space, err := h.q.GetSpaceByID(r.Context(), spaceID)
	if err != nil {
		Error(w, http.StatusNotFound, "space not found")
		return
	}
	if !space.IsActive {
		Error(w, http.StatusConflict, "space is not available for booking")
		return
	}

	hours := endTime.Sub(startTime).Hours()
	if hours < float64(space.MinHours) {
		Error(w, http.StatusBadRequest, "booking duration is below the space minimum")
		return
	}

	// calculate price from space rates — client cannot influence this
	totalPrice := calculatePrice(hours, space.HourlyRate, space.DailyRate, space.MinHours)
	platformFee := int32(0) // TODO: define platform fee rate in config

	// overlap check + insert are atomic: two concurrent requests cannot both pass
	var booking store.Booking
	if err := h.q.ExecTx(r.Context(), func(q store.Querier) error {
		count, err := q.CheckOverlappingBookings(r.Context(), store.CheckOverlappingBookingsParams{
			SpaceID:   spaceID,
			StartTime: startTime,
			EndTime:   endTime,
		})
		if err != nil {
			return err
		}
		if count > 0 {
			return errTimeSlotTaken
		}
		booking, err = q.CreateBooking(r.Context(), store.CreateBookingParams{
			SpaceID:     spaceID,
			RenterID:    claims.ProfileID,
			StartTime:   startTime,
			EndTime:     endTime,
			TotalPrice:  totalPrice,
			PlatformFee: platformFee,
		})
		return err
	}); err != nil {
		if errors.Is(err, errTimeSlotTaken) {
			Error(w, http.StatusConflict, "space is not available for the requested time")
			return
		}
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusCreated, booking)
}

// Get restricts visibility to the renter who made the booking or the owner of the space.
func (h *BookingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())

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

	switch claims.Role {
	case "renter":
		if booking.RenterID != claims.ProfileID {
			Error(w, http.StatusForbidden, "this booking does not belong to you")
			return
		}
	case "owner":
		space, err := h.q.GetSpaceByID(r.Context(), booking.SpaceID)
		if err != nil || space.OwnerID != claims.ProfileID {
			Error(w, http.StatusForbidden, "this booking is not for your space")
			return
		}
	}

	JSON(w, http.StatusOK, booking)
}

// ListBySpace restricts booking visibility to the owner of that space.
func (h *BookingsHandler) ListBySpace(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owner profiles can view space bookings")
		return
	}

	spaceID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid space id")
		return
	}

	space, err := h.q.GetSpaceByID(r.Context(), spaceID)
	if err != nil {
		Error(w, http.StatusNotFound, "space not found")
		return
	}
	if space.OwnerID != claims.ProfileID {
		Error(w, http.StatusForbidden, "this space does not belong to you")
		return
	}

	bookings, err := h.q.ListBookingsBySpace(r.Context(), spaceID)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, bookings)
}

// UpdateStatus enforces role-based status transitions:
// owner (must own the space) → confirmed, completed, cancelled
// renter (must own the booking) → cancelled only
func (h *BookingsHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())

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

	booking, err := h.q.GetBookingByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "booking not found")
		return
	}

	switch claims.Role {
	case "renter":
		if booking.RenterID != claims.ProfileID {
			Error(w, http.StatusForbidden, "this booking does not belong to you")
			return
		}
		if body.Status != store.BookingStatusCancelled {
			Error(w, http.StatusForbidden, "renters can only cancel bookings")
			return
		}
	case "owner":
		space, err := h.q.GetSpaceByID(r.Context(), booking.SpaceID)
		if err != nil || space.OwnerID != claims.ProfileID {
			Error(w, http.StatusForbidden, "this booking is not for your space")
			return
		}
		if body.Status == store.BookingStatusPending {
			Error(w, http.StatusBadRequest, "cannot revert a booking to pending")
			return
		}
	default:
		Error(w, http.StatusForbidden, "unknown role")
		return
	}

	updated, err := h.q.UpdateBookingStatus(r.Context(), store.UpdateBookingStatusParams{
		ID:     id,
		Status: body.Status,
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, updated)
}

type UpdateStatusRequest struct {
	Status store.BookingStatus `json:"status"`
}

// calculatePrice derives the total in satang from space rates and booking duration.
// Bookings under 24h are billed hourly (rounded up, minimum min_hours).
// Bookings 24h or longer are billed daily (rounded up to the next full day).
func calculatePrice(hours float64, hourlyRate, dailyRate, minHours int32) int32 {
	if hours < 24 {
		billable := int32(math.Ceil(hours))
		if billable < minHours {
			billable = minHours
		}
		return billable * hourlyRate
	}
	days := int32(math.Ceil(hours / 24))
	return days * dailyRate
}
