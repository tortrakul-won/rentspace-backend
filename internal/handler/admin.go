package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"rentspace/backend/internal/store"
)

type AdminHandler struct {
	q store.Store
}

func NewAdminHandler(q store.Store) *AdminHandler {
	return &AdminHandler{q: q}
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

// Approve confirms a payment_pending booking → confirmed.
// Also cancels any other overlapping bookings from the same renter and sends notifications.
func (h *AdminHandler) Approve(w http.ResponseWriter, r *http.Request) {
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
	if booking.Status != store.BookingStatusPaymentPending {
		Error(w, http.StatusUnprocessableEntity, "booking must be in payment_pending state to approve")
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

		// Supersede previous notifications then notify renter of confirmation
		if err := q.SupersedeNotificationsByBooking(r.Context(), updated.ID); err != nil {
			log.Printf("supersede notifications for booking %s: %v", updated.ID, err)
		}
		payload, _ := json.Marshal(map[string]string{"booking_id": updated.ID.String(), "space_name": sp.Name})
		_, _ = q.CreateNotification(r.Context(), store.CreateNotificationParams{
			ProfileID: booking.RenterID,
			Type:      "booking_confirmed",
			Payload:   payload,
			BookingID: uuid.NullUUID{UUID: updated.ID, Valid: true},
		})

		// Notify renter of each auto-cancelled backup booking
		for _, cb := range cancelled {
			if err := q.SupersedeNotificationsByBooking(r.Context(), cb.ID); err != nil {
				log.Printf("supersede notifications for booking %s: %v", cb.ID, err)
			}
			cp, _ := json.Marshal(map[string]string{"booking_id": cb.ID.String(), "space_name": sp.Name})
			_, _ = q.CreateNotification(r.Context(), store.CreateNotificationParams{
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

// Reject cancels a payment_pending booking and notifies the renter.
func (h *AdminHandler) Reject(w http.ResponseWriter, r *http.Request) {
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
	if booking.Status != store.BookingStatusPaymentPending {
		Error(w, http.StatusUnprocessableEntity, "booking must be in payment_pending state to reject")
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

		if err := q.SupersedeNotificationsByBooking(r.Context(), updated.ID); err != nil {
			log.Printf("supersede notifications for booking %s: %v", updated.ID, err)
		}
		payload, _ := json.Marshal(map[string]string{"booking_id": updated.ID.String(), "space_name": sp.Name})
		_, _ = q.CreateNotification(r.Context(), store.CreateNotificationParams{
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
