package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
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

	// 30-min boundary check
	details := map[string]string{}
	if startTime.Minute() != 0 && startTime.Minute() != 30 {
		details["start_time"] = "must be on a 30-minute boundary (:00 or :30)"
	}
	if endTime.Minute() != 0 && endTime.Minute() != 30 {
		details["end_time"] = "must be on a 30-minute boundary (:00 or :30)"
	}
	if len(details) > 0 {
		ValidationError(w, "invalid booking times", details)
		return
	}

	if !startTime.After(time.Now()) {
		Error(w, http.StatusUnprocessableEntity, "start_time must be in the future")
		return
	}
	if !endTime.After(startTime) {
		Error(w, http.StatusUnprocessableEntity, "end_time must be after start_time")
		return
	}

	// Same-day check (no midnight crossing)
	startLocal := startTime.In(startTime.Location())
	endLocal := endTime.In(startTime.Location())
	if startLocal.Year() != endLocal.Year() || startLocal.YearDay() != endLocal.YearDay() {
		Error(w, http.StatusUnprocessableEntity, "booking cannot cross midnight")
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

	durationMins := int32(endTime.Sub(startTime).Minutes())

	// Min duration check
	if durationMins < space.MinMinutes {
		minLabel := formatDurationMins(space.MinMinutes)
		ValidationError(w, "booking duration too short", map[string]string{
			"duration": "minimum booking is " + minLabel,
		})
		return
	}

	// Max booking minutes — per space (if set) and system_config hard cap
	sysMaxStr, err := h.q.GetSystemConfig(r.Context(), "max_booking_minutes")
	if err != nil {
		ServerError(w, r, err)
		return
	}
	sysMax, _ := strconv.Atoi(sysMaxStr)
	if sysMax > 0 && durationMins > int32(sysMax) {
		ValidationError(w, "booking duration too long", map[string]string{
			"duration": "exceeds platform maximum of " + formatDurationMins(int32(sysMax)),
		})
		return
	}
	if space.MaxBookingMinutes.Valid && durationMins > space.MaxBookingMinutes.Int32 {
		ValidationError(w, "booking duration too long", map[string]string{
			"duration": "exceeds space maximum of " + formatDurationMins(space.MaxBookingMinutes.Int32),
		})
		return
	}

	// Open hours check — must have availability row for that day of week
	dow := int16(startTime.Weekday())
	avail, err := h.q.GetSpaceAvailability(r.Context(), spaceID)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	var slot *store.SpaceAvailability
	for i := range avail {
		if avail[i].DayOfWeek == dow {
			slot = &avail[i]
			break
		}
	}
	if slot == nil {
		Error(w, http.StatusUnprocessableEntity, "space is closed on that day")
		return
	}

	// Open hours boundary check
	openMins := parseTimeMins(slot.OpenTime)
	closeMins := parseTimeMins(slot.CloseTime)
	startMins := startTime.Hour()*60 + startTime.Minute()
	endMins := endTime.Hour()*60 + endTime.Minute()
	if startMins < openMins {
		ValidationError(w, "booking outside open hours", map[string]string{
			"start_time": "space opens at " + slot.OpenTime,
		})
		return
	}
	if endMins > closeMins {
		ValidationError(w, "booking outside open hours", map[string]string{
			"end_time": "space closes at " + slot.CloseTime,
		})
		return
	}

	// min_notice_hours check
	noticeHours := space.MinNoticeHours
	if time.Until(startTime) < time.Duration(noticeHours)*time.Hour {
		ValidationError(w, "booking too soon", map[string]string{
			"start_time": "must be at least " + strconv.Itoa(int(noticeHours)) + " hour(s) from now",
		})
		return
	}

	// Duplicate active booking per renter per space
	activeCount, err := h.q.CountActiveBookingsByRenterForSpace(r.Context(), store.CountActiveBookingsByRenterForSpaceParams{
		RenterID: claims.ProfileID,
		SpaceID:  spaceID,
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}
	if activeCount > 0 {
		Error(w, http.StatusConflict, "you already have an active booking for this space")
		return
	}

	// Pending cap check
	pendingMaxStr, err := h.q.GetSystemConfig(r.Context(), "max_pending_bookings_per_renter")
	if err != nil {
		ServerError(w, r, err)
		return
	}
	pendingMax, _ := strconv.Atoi(pendingMaxStr)
	pendingCount, err := h.q.CountPendingBookingsByRenter(r.Context(), claims.ProfileID)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	if pendingMax > 0 && pendingCount >= int64(pendingMax) {
		Error(w, http.StatusUnprocessableEntity, "pending booking limit reached; confirm or cancel existing bookings first")
		return
	}

	// Calculate price including weekend surcharge
	hours := endTime.Sub(startTime).Hours()
	isWeekend := startTime.Weekday() == time.Saturday || startTime.Weekday() == time.Sunday
	totalPrice := calculatePrice(hours, space.HourlyRate, space.DailyRate, space.MinMinutes, space.WeekendSurchargePct, isWeekend)

	// platform fee from system_config
	platformFeePctStr, _ := h.q.GetSystemConfig(r.Context(), "platform_fee_pct")
	platformFeePct, _ := strconv.Atoi(platformFeePctStr)
	platformFee := int32(math.Round(float64(totalPrice) * float64(platformFeePct) / 100))

	// expires_at = start_time - min_notice_hours
	expiresAt := startTime.Add(-time.Duration(noticeHours) * time.Hour)

	var booking store.Booking
	if err := h.q.ExecTx(r.Context(), func(q store.Querier) error {
		// overlap check vs existing bookings
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

		// overlap check vs space blocks
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

		var headcount sql.NullInt32
		if body.Headcount != nil {
			headcount = sql.NullInt32{Int32: int32(*body.Headcount), Valid: true}
		}
		var notes sql.NullString
		if body.Notes != "" {
			notes = sql.NullString{String: body.Notes, Valid: true}
		}

		booking, err = q.CreateBooking(r.Context(), store.CreateBookingParams{
			SpaceID:     spaceID,
			RenterID:    claims.ProfileID,
			StartTime:   startTime,
			EndTime:     endTime,
			TotalPrice:  totalPrice,
			PlatformFee: platformFee,
			Headcount:   headcount,
			Notes:       notes,
			ExpiresAt:   sql.NullTime{Time: expiresAt, Valid: true},
		})
		if err != nil {
			return err
		}

		// Notify the space owner
		ownerID := space.OwnerID
		payload, _ := json.Marshal(map[string]string{
			"booking_id": booking.ID.String(),
			"space_name": space.Name,
		})
		_, _ = q.CreateNotification(r.Context(), store.CreateNotificationParams{
			ProfileID: ownerID,
			Type:      "booking_request",
			Payload:   payload,
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
// owner: pending → confirmed, pending/confirmed → cancelled
// renter: pending/confirmed → cancelled only
// completed is automatic only — no manual trigger allowed.
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
		// owner can only: confirmed (accept), cancelled (decline)
		if body.Status != store.BookingStatusConfirmed && body.Status != store.BookingStatusCancelled {
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
	if !isValidTransition(current, next) {
		Error(w, http.StatusUnprocessableEntity, "invalid status transition from "+string(current)+" to "+string(next))
		return
	}

	var updated store.Booking
	if err := h.q.ExecTx(r.Context(), func(q store.Querier) error {
		var err error
		updated, err = q.UpdateBookingStatus(r.Context(), store.UpdateBookingStatusParams{
			ID:     id,
			Status: next,
		})
		if err != nil {
			return err
		}

		// On confirm: auto-cancel other overlapping pending bookings by same renter
		if next == store.BookingStatusConfirmed {
			cancelled, err := q.CancelOverlappingPendingBookings(r.Context(), store.CancelOverlappingPendingBookingsParams{
				RenterID:  booking.RenterID,
				ID:        id,
				StartTime: booking.StartTime,
				EndTime:   booking.EndTime,
			})
			if err != nil {
				return err
			}

			// Notify renter of confirmation
			payload, _ := json.Marshal(map[string]string{
				"booking_id": updated.ID.String(),
			})
			_, _ = q.CreateNotification(r.Context(), store.CreateNotificationParams{
				ProfileID: booking.RenterID,
				Type:      "booking_confirmed",
				Payload:   payload,
			})

			// Notify renter of each auto-cancelled backup booking
			for _, cb := range cancelled {
				cp, _ := json.Marshal(map[string]string{
					"booking_id": cb.ID.String(),
				})
				_, _ = q.CreateNotification(r.Context(), store.CreateNotificationParams{
					ProfileID: booking.RenterID,
					Type:      "backup_booking_cancelled",
					Payload:   cp,
				})
			}
		}

		// On cancellation: notify the other party
		if next == store.BookingStatusCancelled {
			payload, _ := json.Marshal(map[string]string{
				"booking_id": updated.ID.String(),
			})
			if claims.Role == "renter" {
				// renter cancels → notify owner
				sp, err := q.GetSpaceByID(r.Context(), booking.SpaceID)
				if err == nil {
					_, _ = q.CreateNotification(r.Context(), store.CreateNotificationParams{
						ProfileID: sp.OwnerID,
						Type:      "booking_cancelled_by_renter",
						Payload:   payload,
					})
				}
			} else {
				// owner declines/cancels → notify renter
				_, _ = q.CreateNotification(r.Context(), store.CreateNotificationParams{
					ProfileID: booking.RenterID,
					Type:      "booking_cancelled_by_owner",
					Payload:   payload,
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
	bookings, err := h.q.ListBookingsByRenter(r.Context(), claims.ProfileID)
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
	bookings, err := h.q.ListBookingsByOwner(r.Context(), claims.ProfileID)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, nonNil(bookings))
}

// isValidTransition returns true if the status transition is allowed by the state machine.
func isValidTransition(from, to store.BookingStatus) bool {
	switch from {
	case store.BookingStatusPending:
		return to == store.BookingStatusConfirmed || to == store.BookingStatusCancelled
	case store.BookingStatusConfirmed:
		return to == store.BookingStatusCancelled
	default:
		return false
	}
}

// calculatePrice derives the total in satang from space rates and booking duration.
// Bookings under 24h are billed hourly (rounded up, minimum ceil(min_minutes/60)).
// Bookings 24h or longer are billed daily (rounded up to the next full day).
// Weekend surcharge is applied when isWeekend is true.
func calculatePrice(hours float64, hourlyRate, dailyRate, minMinutes, weekendSurchargePct int32, isWeekend bool) int32 {
	var base int32
	if hours < 24 {
		billable := int32(math.Ceil(hours))
		minBillableHours := int32(math.Ceil(float64(minMinutes) / 60))
		if billable < minBillableHours {
			billable = minBillableHours
		}
		base = billable * hourlyRate
	} else {
		days := int32(math.Ceil(hours / 24))
		base = days * dailyRate
	}
	if isWeekend && weekendSurchargePct > 0 {
		surcharge := int32(math.Round(float64(base) * float64(weekendSurchargePct) / 100))
		base += surcharge
	}
	return base
}

// parseTimeMins converts "HH:MM" string to minutes since midnight.
func parseTimeMins(t string) int {
	if len(t) < 5 {
		return 0
	}
	h, _ := strconv.Atoi(t[:2])
	m, _ := strconv.Atoi(t[3:5])
	return h*60 + m
}

func formatDurationMins(mins int32) string {
	if mins < 60 {
		return strconv.Itoa(int(mins)) + " min"
	}
	h := mins / 60
	m := mins % 60
	if m == 0 {
		return strconv.Itoa(int(h)) + " hr"
	}
	return strconv.Itoa(int(h)) + " hr " + strconv.Itoa(int(m)) + " min"
}
