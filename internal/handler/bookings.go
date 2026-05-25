package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	bk "rentspace/backend/internal/booking"
	"rentspace/backend/internal/hub"
	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

var errTimeSlotTaken = errors.New("time slot taken")

type BookingsHandler struct {
	q   store.Store
	hub *hub.Hub
}

func NewBookingsHandler(q store.Store, h *hub.Hub) *BookingsHandler {
	return &BookingsHandler{q: q, hub: h}
}

// Create validates all booking rules and creates the booking atomically.
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

	// Fetch all deps before calling policy.
	space, err := h.q.GetSpaceByID(r.Context(), spaceID)
	if err != nil {
		Error(w, http.StatusNotFound, "space not found")
		return
	}
	avail, err := h.q.GetSpaceAvailability(r.Context(), spaceID)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	configs, err := h.q.GetSystemConfigMultiple(r.Context(), []string{
		"max_booking_minutes",
		"platform_fee_pct",
		"max_pending_bookings_per_renter",
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}
	activeCount, err := h.q.CountActiveBookingsByRenterForSpace(r.Context(), store.CountActiveBookingsByRenterForSpaceParams{
		RenterID: claims.ProfileID,
		SpaceID:  spaceID,
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}
	pendingCount, err := h.q.CountPendingBookingsByRenter(r.Context(), claims.ProfileID)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	maxBookingMins, _ := strconv.Atoi(configs["max_booking_minutes"])
	platformFeePct, _ := strconv.Atoi(configs["platform_fee_pct"])
	maxPendingCap, _ := strconv.Atoi(configs["max_pending_bookings_per_renter"])

	var headcount *int32
	if body.Headcount != nil {
		v := int32(*body.Headcount)
		headcount = &v
	}

	proposal, polErr := bk.Validate(bk.Request{
		StartTime: startTime,
		EndTime:   endTime,
		Headcount: headcount,
		Notes:     body.Notes,
	}, bk.Inputs{
		Space:        space,
		Availability: avail,
		Config: bk.Config{
			MaxBookingMins: int32(maxBookingMins),
			PlatformFeePct: int32(platformFeePct),
			MaxPendingCap:  int32(maxPendingCap),
		},
		PendingCount: pendingCount,
		ActiveCount:  activeCount,
		Now:          time.Now(),
	})
	if polErr != nil {
		status := polErr.Status
		if len(polErr.Details) > 0 {
			ValidationError(w, polErr.Message, polErr.Details)
		} else {
			Error(w, status, polErr.Message)
		}
		return
	}

	var created store.Booking
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
		blockCount, err := q.CheckOverlappingSpaceBlocks(r.Context(), store.CheckOverlappingSpaceBlocksParams{
			SpaceID:   spaceID,
			StartTime: startTime,
			EndTime:   endTime,
		})
		if err != nil {
			return err
		}
		if blockCount > 0 {
			return errTimeSlotTaken
		}

		var sqlHeadcount sql.NullInt32
		if proposal.Headcount != nil {
			sqlHeadcount = sql.NullInt32{Int32: *proposal.Headcount, Valid: true}
		}
		var sqlNotes sql.NullString
		if proposal.Notes != "" {
			sqlNotes = sql.NullString{String: proposal.Notes, Valid: true}
		}

		created, err = q.CreateBooking(r.Context(), store.CreateBookingParams{
			SpaceID:     spaceID,
			RenterID:    claims.ProfileID,
			StartTime:   proposal.StartTime,
			EndTime:     proposal.EndTime,
			TotalPrice:  proposal.TotalPrice,
			PlatformFee: proposal.PlatformFee,
			Headcount:   sqlHeadcount,
			Notes:       sqlNotes,
			ExpiresAt:   sql.NullTime{Time: proposal.ExpiresAt, Valid: true},
		})
		if err != nil {
			return err
		}

		payload, _ := json.Marshal(map[string]string{
			"booking_id": created.ID.String(),
			"space_name": space.Name,
		})
		pushNotification(r.Context(), q, h.hub, store.CreateNotificationParams{
			ProfileID: space.OwnerID,
			Type:      "booking_request",
			Payload:   payload,
			BookingID: uuid.NullUUID{UUID: created.ID, Valid: true},
		})
		return nil
	}); err != nil {
		if errors.Is(err, errTimeSlotTaken) {
			Error(w, http.StatusConflict, "space is not available for the requested time")
			return
		}
		ServerError(w, r, err)
		return
	}

	JSON(w, http.StatusCreated, created)
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

	page, limit := parsePagination(r)
	offset := (page - 1) * limit

	bookings, err := h.q.ListBookingsBySpacePaginated(r.Context(), store.ListBookingsBySpacePaginatedParams{
		SpaceID: spaceID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}
	total, err := h.q.CountBookingsBySpace(r.Context(), spaceID)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, Page[store.Booking]{
		Data:    nonNil(bookings),
		Total:   total,
		Page:    page,
		Limit:   limit,
		HasMore: int64(offset)+int64(len(bookings)) < total,
	})
}

// UpdateStatus enforces role-based state machine transitions.
// owner: pending → awaiting_payment (accept), pending/awaiting_payment/payment_review/confirmed → cancelled
// renter: pending/awaiting_payment/payment_review/confirmed → cancelled only
// completed is automatic only — no manual trigger allowed.
// Admin approval (payment_review → confirmed) is handled by the admin handler.
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

	// completed is automatic only
	if body.Status == store.BookingStatusCompleted {
		Error(w, http.StatusForbidden, "completed status is set automatically when the booking end time passes")
		return
	}
	// confirmed requires admin approval via /admin/bookings/:id/approve
	if body.Status == store.BookingStatusConfirmed {
		Error(w, http.StatusForbidden, "payment confirmation is handled by admin")
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
		// owner can only: awaiting_payment (accept), cancelled (decline)
		if body.Status != store.BookingStatusAwaitingPayment && body.Status != store.BookingStatusCancelled {
			Error(w, http.StatusBadRequest, "invalid status transition")
			return
		}
	default:
		Error(w, http.StatusForbidden, "unknown role")
		return
	}

	// Validate state machine transitions
	current := booking.Status
	next := body.Status
	if !bk.IsValidTransition(current, next) {
		Error(w, http.StatusUnprocessableEntity, "invalid status transition from "+string(current)+" to "+string(next))
		return
	}

	var updated store.Booking
	if err := h.q.ExecTx(r.Context(), func(q store.Querier) error {
		var err error
		cancelReason := sql.NullString{}
		if next == store.BookingStatusCancelled {
			if claims.Role == "renter" {
				cancelReason = sql.NullString{String: "renter_cancelled", Valid: true}
			} else {
				cancelReason = sql.NullString{String: "owner_declined", Valid: true}
			}
		}
		updated, err = q.UpdateBookingStatus(r.Context(), store.UpdateBookingStatusParams{
			ID:           id,
			Status:       next,
			CancelReason: cancelReason,
		})
		if err != nil {
			return err
		}

		// Supersede previous notifications for this booking before creating new one
		if err := q.SupersedeNotificationsByBooking(r.Context(), updated.ID); err != nil {
			log.Printf("supersede notifications for booking %s: %v", updated.ID, err)
		}

		sp, _ := q.GetSpaceByID(r.Context(), booking.SpaceID)

		// Owner accepts → notify renter to upload payment slip
		if next == store.BookingStatusAwaitingPayment {
			payload, _ := json.Marshal(map[string]string{
				"booking_id": updated.ID.String(),
				"space_name": sp.Name,
			})
			pushNotification(r.Context(), q, h.hub, store.CreateNotificationParams{
				ProfileID: booking.RenterID,
				Type:      "payment_required",
				Payload:   payload,
				BookingID: uuid.NullUUID{UUID: updated.ID, Valid: true},
			})
		}

		// On cancellation: notify the other party
		if next == store.BookingStatusCancelled {
			payload, _ := json.Marshal(map[string]string{
				"booking_id": updated.ID.String(),
				"space_name": sp.Name,
			})
			if claims.Role == "renter" {
				// renter cancels → notify owner
				pushNotification(r.Context(), q, h.hub, store.CreateNotificationParams{
					ProfileID: sp.OwnerID,
					Type:      "booking_cancelled_by_renter",
					Payload:   payload,
					BookingID: uuid.NullUUID{UUID: updated.ID, Valid: true},
				})
			} else {
				// owner declines/cancels → notify renter
				pushNotification(r.Context(), q, h.hub, store.CreateNotificationParams{
					ProfileID: booking.RenterID,
					Type:      "booking_cancelled_by_owner",
					Payload:   payload,
					BookingID: uuid.NullUUID{UUID: updated.ID, Valid: true},
				})
			}
		}

		return nil
	}); err != nil {
		ServerError(w, r, err)
		return
	}

	JSON(w, http.StatusOK, updated)
}

type UpdateStatusRequest struct {
	Status store.BookingStatus `json:"status"`
}

func (h *BookingsHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "renter" {
		Error(w, http.StatusForbidden, "only renter profiles can list their bookings")
		return
	}
	bookings, err := h.q.ListBookingsByRenterEnriched(r.Context(), claims.ProfileID)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, nonNil(bookings))
}

func (h *BookingsHandler) ListMineOwner(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owner profiles can list their space bookings")
		return
	}
	bookings, err := h.q.ListBookingsByOwnerEnriched(r.Context(), claims.ProfileID)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, nonNil(bookings))
}

