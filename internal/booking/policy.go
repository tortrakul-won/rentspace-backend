package booking

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"rentspace/backend/internal/store"
)

// Request holds caller-parsed fields for a new Booking.
// Handlers parse RFC3339 strings and UUIDs before calling Validate.
type Request struct {
	StartTime time.Time
	EndTime   time.Time
	Headcount *int32
	Notes     string
}

// Config holds system-level settings resolved from system_config before calling Validate.
type Config struct {
	MaxBookingMins int32
	PlatformFeePct int32
	MaxPendingCap  int32
}

// Inputs is the complete pre-fetched data set needed to validate a Booking.
// All fields are resolved by the handler; Validate is pure (no IO).
type Inputs struct {
	Space        store.Space
	Availability []store.SpaceAvailability
	Config       Config
	PendingCount int64
	ActiveCount  int64
	Now          time.Time
}

// Proposal is returned by a successful Validate call — all values needed
// to call store.CreateBooking without further computation.
type Proposal struct {
	StartTime   time.Time
	EndTime     time.Time
	TotalPrice  int32
	PlatformFee int32
	ExpiresAt   time.Time
	Headcount   *int32
	Notes       string
}

// ValidationError carries a human-readable message, optional per-field details,
// and the HTTP status the handler should use (0 defaults to 422).
type ValidationError struct {
	Message string
	Details map[string]string
	Status  int
}

func (e *ValidationError) Error() string { return e.Message }

func conflict(msg string) *ValidationError {
	return &ValidationError{Message: msg, Status: http.StatusConflict}
}

func unprocessable(msg string, details map[string]string) *ValidationError {
	return &ValidationError{Message: msg, Details: details, Status: http.StatusUnprocessableEntity}
}

// Validate checks all Booking business rules in order and, if valid, returns a
// Proposal with computed total_price, platform_fee, and expires_at.
//
//  1. 30-minute time boundary
//  2. start_time in the future
//  3. end_time > start_time
//  4. Same-day (no midnight crossing)
//  5. Space is active
//  6. Duration vs space min / system+space max
//  7. Open hours (SpaceAvailability schedule)
//  8. Min-notice hours
//  9. No duplicate active Booking for same Renter+Space
//  10. Pending-booking cap
func Validate(req Request, in Inputs) (*Proposal, *ValidationError) {
	// 1. 30-minute boundary
	details := map[string]string{}
	if req.StartTime.Minute() != 0 && req.StartTime.Minute() != 30 {
		details["start_time"] = "must be on a 30-minute boundary (:00 or :30)"
	}
	if req.EndTime.Minute() != 0 && req.EndTime.Minute() != 30 {
		details["end_time"] = "must be on a 30-minute boundary (:00 or :30)"
	}
	if len(details) > 0 {
		return nil, unprocessable("invalid booking times", details)
	}

	// 2. Future start
	if !req.StartTime.After(in.Now) {
		return nil, unprocessable("start_time must be in the future", nil)
	}

	// 3. end > start
	if !req.EndTime.After(req.StartTime) {
		return nil, unprocessable("end_time must be after start_time", nil)
	}

	// 4. Same-day (no midnight crossing)
	sl := req.StartTime.In(req.StartTime.Location())
	el := req.EndTime.In(req.StartTime.Location())
	if sl.Year() != el.Year() || sl.YearDay() != el.YearDay() {
		return nil, unprocessable("booking cannot cross midnight", nil)
	}

	// 5. Space active
	if !in.Space.IsActive {
		return nil, conflict("space is not available for booking")
	}

	// 6. Duration
	durationMins := int32(req.EndTime.Sub(req.StartTime).Minutes())
	if durationMins < in.Space.MinMinutes {
		return nil, unprocessable("booking duration too short", map[string]string{
			"duration": "minimum booking is " + formatDurationMins(in.Space.MinMinutes),
		})
	}
	if in.Config.MaxBookingMins > 0 && durationMins > in.Config.MaxBookingMins {
		return nil, unprocessable("booking duration too long", map[string]string{
			"duration": "exceeds platform maximum of " + formatDurationMins(in.Config.MaxBookingMins),
		})
	}
	if in.Space.MaxBookingMinutes.Valid && durationMins > in.Space.MaxBookingMinutes.Int32 {
		return nil, unprocessable("booking duration too long", map[string]string{
			"duration": "exceeds space maximum of " + formatDurationMins(in.Space.MaxBookingMinutes.Int32),
		})
	}

	// 7. Open hours
	dow := int16(req.StartTime.Weekday())
	var slot *store.SpaceAvailability
	for i := range in.Availability {
		if in.Availability[i].DayOfWeek == dow {
			slot = &in.Availability[i]
			break
		}
	}
	if slot == nil {
		return nil, unprocessable("space is closed on that day", nil)
	}
	openMins := parseTimeMins(slot.OpenTime)
	closeMins := parseTimeMins(slot.CloseTime)
	startMins := req.StartTime.Hour()*60 + req.StartTime.Minute()
	endMins := req.EndTime.Hour()*60 + req.EndTime.Minute()
	if startMins < openMins {
		return nil, unprocessable("booking outside open hours", map[string]string{
			"start_time": "space opens at " + slot.OpenTime,
		})
	}
	if endMins > closeMins {
		return nil, unprocessable("booking outside open hours", map[string]string{
			"end_time": "space closes at " + slot.CloseTime,
		})
	}

	// 8. Min-notice
	noticeHours := in.Space.MinNoticeHours
	if in.Now.Add(time.Duration(noticeHours) * time.Hour).After(req.StartTime) {
		return nil, unprocessable("booking too soon", map[string]string{
			"start_time": "must be at least " + strconv.Itoa(int(noticeHours)) + " hour(s) from now",
		})
	}

	// 9. Duplicate active booking per Renter+Space
	if in.ActiveCount > 0 {
		return nil, conflict("you already have an active booking for this space")
	}

	// 10. Pending cap
	if in.Config.MaxPendingCap > 0 && in.PendingCount >= int64(in.Config.MaxPendingCap) {
		return nil, unprocessable("pending booking limit reached; confirm or cancel existing bookings first", nil)
	}

	// Compute price and derived fields
	hours := req.EndTime.Sub(req.StartTime).Hours()
	isWeekend := req.StartTime.Weekday() == time.Saturday || req.StartTime.Weekday() == time.Sunday
	totalPrice := calculatePrice(hours, in.Space.HourlyRate, in.Space.DailyRate, in.Space.MinMinutes, in.Space.WeekendSurchargePct, isWeekend)
	platformFee := int32(math.Round(float64(totalPrice) * float64(in.Config.PlatformFeePct) / 100))
	expiresAt := req.StartTime.Add(-time.Duration(noticeHours) * time.Hour)

	return &Proposal{
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		TotalPrice:  totalPrice,
		PlatformFee: platformFee,
		ExpiresAt:   expiresAt,
		Headcount:   req.Headcount,
		Notes:       req.Notes,
	}, nil
}

// IsValidTransition reports whether a Booking status transition is permitted
// by the state machine.
func IsValidTransition(from, to store.BookingStatus) bool {
	switch from {
	case store.BookingStatusPending:
		return to == store.BookingStatusAwaitingPayment || to == store.BookingStatusCancelled
	case store.BookingStatusAwaitingPayment:
		return to == store.BookingStatusPaymentReview || to == store.BookingStatusCancelled
	case store.BookingStatusPaymentReview:
		return to == store.BookingStatusConfirmed || to == store.BookingStatusAwaitingPayment || to == store.BookingStatusCancelled
	case store.BookingStatusConfirmed:
		return to == store.BookingStatusCancelled
	default:
		return false
	}
}

// calculatePrice derives the total in satang from Space rates and booking duration.
// Bookings under 24h: billed hourly (rounded up, minimum ceil(min_minutes/60)).
// Bookings 24h+: billed daily (rounded up to next full day).
// Weekend surcharge applied when isWeekend is true.
func calculatePrice(hours float64, hourlyRate, dailyRate, minMinutes, weekendSurchargePct int32, isWeekend bool) int32 {
	var base int32
	if hours < 24 {
		billable := int32(math.Ceil(hours))
		minBillable := int32(math.Ceil(float64(minMinutes) / 60))
		if billable < minBillable {
			billable = minBillable
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

// parseTimeMins converts "HH:MM" to minutes since midnight.
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
