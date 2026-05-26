package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"rentspace/backend/internal/hub"
	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

type AdminHandler struct {
	q   store.Store
	hub *hub.Hub
}

func NewAdminHandler(q store.Store, h *hub.Hub) *AdminHandler {
	return &AdminHandler{q: q, hub: h}
}

// ListPaymentPending returns all bookings awaiting payment verification.
func (h *AdminHandler) ListPaymentPending(w http.ResponseWriter, r *http.Request) {
	bookings, err := h.q.ListPaymentPendingBookings(r.Context())
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, nonNil(bookings))
}

// GetBookingDetail returns a fully-enriched booking for the admin review page.
func (h *AdminHandler) GetBookingDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	row, err := h.q.GetAdminBookingDetail(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "booking not found")
		return
	}
	JSON(w, http.StatusOK, adminBookingDetailToResponse(row))
}

// Approve confirms a payment_review booking → confirmed.
// Also cancels any other overlapping bookings from the same renter and sends notifications.
func (h *AdminHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	claims := middleware.ClaimsFromCtx(r.Context())

	booking, err := h.q.GetBookingByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "booking not found")
		return
	}
	if booking.Status != store.BookingStatusPaymentReview {
		Error(w, http.StatusUnprocessableEntity, "booking must be in payment_review state to approve")
		return
	}

	renterProfile, err := h.q.GetProfileByID(r.Context(), booking.RenterID)
	if err == nil && renterProfile.UserID == claims.UserID {
		Error(w, http.StatusForbidden, "cannot approve your own booking")
		return
	}

	var updated store.Booking
	if err := h.q.ExecTx(r.Context(), func(q store.Querier) error {
		var err error
		updated, err = q.UpdateBookingStatus(r.Context(), store.UpdateBookingStatusParams{
			ID:     id,
			Status: store.BookingStatusConfirmed,
			CancelReason: sql.NullString{},
		})
		if err != nil {
			return err
		}

		// Cancel overlapping pending/payment_pending bookings from the same renter
		cancelled, err := q.CancelOverlappingPendingBookings(r.Context(), store.CancelOverlappingPendingBookingsParams{
			RenterID:  booking.RenterID,
			ID:        id,
			StartTime: booking.StartTime,
			EndTime:   booking.EndTime,
		})
		if err != nil {
			return err
		}

		sp, _ := q.GetSpaceByID(r.Context(), booking.SpaceID)

		payload, _ := json.Marshal(map[string]string{"booking_id": updated.ID.String(), "space_name": sp.Name})
		pushNotification(r.Context(), q, h.hub, store.CreateNotificationParams{
			ProfileID: booking.RenterID,
			Type:      "booking_confirmed",
			Payload:   payload,
			BookingID: uuid.NullUUID{UUID: updated.ID, Valid: true},
		})

		// Notify renter of each auto-cancelled backup booking
		for _, cb := range cancelled {
			cp, _ := json.Marshal(map[string]string{"booking_id": cb.ID.String(), "space_name": sp.Name})
			pushNotification(r.Context(), q, h.hub, store.CreateNotificationParams{
				ProfileID: booking.RenterID,
				Type:      "backup_booking_cancelled",
				Payload:   cp,
				BookingID: uuid.NullUUID{UUID: cb.ID, Valid: true},
			})
		}

		return nil
	}); err != nil {
		ServerError(w, r, err)
		return
	}

	JSON(w, http.StatusOK, updated)
}

// RejectRetry moves a payment_review booking back to awaiting_payment so the renter can retry.
func (h *AdminHandler) RejectRetry(w http.ResponseWriter, r *http.Request) {
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
	if booking.Status != store.BookingStatusPaymentReview {
		Error(w, http.StatusUnprocessableEntity, "booking must be in payment_review state to reject")
		return
	}

	var updated store.Booking
	if err := h.q.ExecTx(r.Context(), func(q store.Querier) error {
		var err error
		updated, err = q.UpdateBookingStatus(r.Context(), store.UpdateBookingStatusParams{
			ID:           id,
			Status:       store.BookingStatusAwaitingPayment,
			CancelReason: sql.NullString{},
		})
		if err != nil {
			return err
		}

		sp, _ := q.GetSpaceByID(r.Context(), booking.SpaceID)

		payload, _ := json.Marshal(map[string]string{"booking_id": updated.ID.String(), "space_name": sp.Name})
		pushNotification(r.Context(), q, h.hub, store.CreateNotificationParams{
			ProfileID: booking.RenterID,
			Type:      "payment_rejected",
			Payload:   payload,
			BookingID: uuid.NullUUID{UUID: updated.ID, Valid: true},
		})

		return nil
	}); err != nil {
		ServerError(w, r, err)
		return
	}

	JSON(w, http.StatusOK, updated)
}

// RejectPermanent cancels a payment_review booking permanently and notifies the renter.
func (h *AdminHandler) RejectPermanent(w http.ResponseWriter, r *http.Request) {
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
	if booking.Status != store.BookingStatusPaymentReview {
		Error(w, http.StatusUnprocessableEntity, "booking must be in payment_review state to reject permanently")
		return
	}

	var updated store.Booking
	if err := h.q.ExecTx(r.Context(), func(q store.Querier) error {
		var err error
		updated, err = q.UpdateBookingStatus(r.Context(), store.UpdateBookingStatusParams{
			ID:           id,
			Status:       store.BookingStatusCancelled,
			CancelReason: sql.NullString{String: "payment_rejected", Valid: true},
		})
		if err != nil {
			return err
		}

		sp, _ := q.GetSpaceByID(r.Context(), booking.SpaceID)

		payload, _ := json.Marshal(map[string]string{"booking_id": updated.ID.String(), "space_name": sp.Name})
		pushNotification(r.Context(), q, h.hub, store.CreateNotificationParams{
			ProfileID: booking.RenterID,
			Type:      "payment_rejected",
			Payload:   payload,
			BookingID: uuid.NullUUID{UUID: updated.ID, Valid: true},
		})

		return nil
	}); err != nil {
		ServerError(w, r, err)
		return
	}

	JSON(w, http.StatusOK, updated)
}
