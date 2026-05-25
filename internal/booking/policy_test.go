package booking_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"rentspace/backend/internal/booking"
	"rentspace/backend/internal/store"
)

// baseSpace returns a sensible default Space for tests.
func baseSpace() store.Space {
	return store.Space{
		ID:                  uuid.New(),
		OwnerID:             uuid.New(),
		IsActive:            true,
		HourlyRate:          500,
		DailyRate:           3000,
		MinMinutes:          60,
		WeekendSurchargePct: 0,
		MinNoticeHours:      0,
		MaxBookingMinutes:   sql.NullInt32{},
	}
}

// baseAvail returns a schedule open Mon–Sun 09:00–18:00.
func baseAvail() []store.SpaceAvailability {
	slots := make([]store.SpaceAvailability, 7)
	for i := range slots {
		slots[i] = store.SpaceAvailability{
			DayOfWeek: int16(i),
			OpenTime:  "09:00",
			CloseTime: "18:00",
		}
	}
	return slots
}

// baseConfig returns permissive system config.
func baseConfig() booking.Config {
	return booking.Config{
		MaxBookingMins: 0, // no cap
		PlatformFeePct: 10,
		MaxPendingCap:  0, // no cap
	}
}

// monday09 returns a Monday at 09:00 UTC well in the future.
func monday09() time.Time {
	// Find next Monday from a fixed reference point.
	ref := time.Date(2030, 1, 7, 9, 0, 0, 0, time.UTC) // 2030-01-07 is a Monday
	return ref
}

func baseInputs(now time.Time) booking.Inputs {
	return booking.Inputs{
		Space:        baseSpace(),
		Availability: baseAvail(),
		Config:       baseConfig(),
		PendingCount: 0,
		ActiveCount:  0,
		Now:          now,
	}
}

func baseRequest(start, end time.Time) booking.Request {
	return booking.Request{StartTime: start, EndTime: end}
}

// --- Validate ---

func TestValidate_Success(t *testing.T) {
	start := monday09()
	end := start.Add(2 * time.Hour)
	now := start.Add(-24 * time.Hour)

	proposal, err := booking.Validate(baseRequest(start, end), baseInputs(now))
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	// 2 hours × 500 satang/hr = 1000; platform fee 10% = 100
	if proposal.TotalPrice != 1000 {
		t.Errorf("TotalPrice: want 1000, got %d", proposal.TotalPrice)
	}
	if proposal.PlatformFee != 100 {
		t.Errorf("PlatformFee: want 100, got %d", proposal.PlatformFee)
	}
}

func TestValidate_StartNotOnBoundary(t *testing.T) {
	start := monday09().Add(15 * time.Minute) // :15 not on boundary
	end := start.Add(2 * time.Hour)
	now := start.Add(-24 * time.Hour)

	_, err := booking.Validate(baseRequest(start, end), baseInputs(now))
	if err == nil {
		t.Fatal("expected validation error")
	}
	if err.Details["start_time"] == "" {
		t.Error("expected start_time field error")
	}
}

func TestValidate_EndNotOnBoundary(t *testing.T) {
	start := monday09()
	end := start.Add(2*time.Hour + 15*time.Minute) // :15 not on boundary
	now := start.Add(-24 * time.Hour)

	_, err := booking.Validate(baseRequest(start, end), baseInputs(now))
	if err == nil {
		t.Fatal("expected validation error")
	}
	if err.Details["end_time"] == "" {
		t.Error("expected end_time field error")
	}
}

func TestValidate_StartInPast(t *testing.T) {
	start := monday09()
	end := start.Add(2 * time.Hour)
	now := start.Add(time.Hour) // now is AFTER start

	_, err := booking.Validate(baseRequest(start, end), baseInputs(now))
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Message != "start_time must be in the future" {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestValidate_EndBeforeStart(t *testing.T) {
	start := monday09()
	end := start.Add(-30 * time.Minute)
	now := start.Add(-24 * time.Hour)

	_, err := booking.Validate(baseRequest(start, end), baseInputs(now))
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Message != "end_time must be after start_time" {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestValidate_MidnightCrossing(t *testing.T) {
	start := monday09()
	end := start.Add(16 * time.Hour) // 09:00 + 16h = 01:00 next day
	now := start.Add(-24 * time.Hour)

	_, err := booking.Validate(baseRequest(start, end), baseInputs(now))
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Message != "booking cannot cross midnight" {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestValidate_SpaceInactive(t *testing.T) {
	start := monday09()
	end := start.Add(2 * time.Hour)
	now := start.Add(-24 * time.Hour)

	in := baseInputs(now)
	in.Space.IsActive = false

	_, err := booking.Validate(baseRequest(start, end), in)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Status != http.StatusConflict {
		t.Errorf("want 409, got %d", err.Status)
	}
}

func TestValidate_DurationTooShort(t *testing.T) {
	start := monday09()
	end := start.Add(30 * time.Minute) // 30 min < MinMinutes=60
	now := start.Add(-24 * time.Hour)

	_, err := booking.Validate(baseRequest(start, end), baseInputs(now))
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Details["duration"] == "" {
		t.Error("expected duration field error")
	}
}

func TestValidate_DurationExceedsSystemCap(t *testing.T) {
	start := monday09()
	end := start.Add(5 * time.Hour)
	now := start.Add(-24 * time.Hour)

	in := baseInputs(now)
	in.Config.MaxBookingMins = 120 // 2 hr cap; booking is 5 hr

	_, err := booking.Validate(baseRequest(start, end), in)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Message != "booking duration too long" {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestValidate_DurationExceedsSpaceCap(t *testing.T) {
	start := monday09()
	end := start.Add(5 * time.Hour)
	now := start.Add(-24 * time.Hour)

	in := baseInputs(now)
	in.Space.MaxBookingMinutes = sql.NullInt32{Int32: 120, Valid: true}

	_, err := booking.Validate(baseRequest(start, end), in)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Message != "booking duration too long" {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestValidate_ClosedOnThatDay(t *testing.T) {
	// monday09() is a Monday (weekday 1). Remove Monday from schedule.
	start := monday09()
	end := start.Add(2 * time.Hour)
	now := start.Add(-24 * time.Hour)

	in := baseInputs(now)
	avail := make([]store.SpaceAvailability, 0)
	for _, s := range baseAvail() {
		if s.DayOfWeek != int16(time.Monday) {
			avail = append(avail, s)
		}
	}
	in.Availability = avail

	_, err := booking.Validate(baseRequest(start, end), in)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Message != "space is closed on that day" {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestValidate_StartBeforeOpen(t *testing.T) {
	// 08:00 is before the 09:00 open time.
	start := time.Date(2030, 1, 7, 8, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	now := start.Add(-24 * time.Hour)

	_, err := booking.Validate(baseRequest(start, end), baseInputs(now))
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Details["start_time"] == "" {
		t.Error("expected start_time field error")
	}
}

func TestValidate_EndAfterClose(t *testing.T) {
	// 16:30 start + 2h = 18:30, but space closes at 18:00.
	start := time.Date(2030, 1, 7, 16, 30, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	now := start.Add(-24 * time.Hour)

	_, err := booking.Validate(baseRequest(start, end), baseInputs(now))
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Details["end_time"] == "" {
		t.Error("expected end_time field error")
	}
}

func TestValidate_TooSoonMinNotice(t *testing.T) {
	start := monday09()
	end := start.Add(2 * time.Hour)
	// now is only 1h before start, but min_notice_hours = 2
	now := start.Add(-time.Hour)

	in := baseInputs(now)
	in.Space.MinNoticeHours = 2

	_, err := booking.Validate(baseRequest(start, end), in)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Details["start_time"] == "" {
		t.Error("expected start_time field error")
	}
}

func TestValidate_DuplicateActiveBooking(t *testing.T) {
	start := monday09()
	end := start.Add(2 * time.Hour)
	now := start.Add(-24 * time.Hour)

	in := baseInputs(now)
	in.ActiveCount = 1

	_, err := booking.Validate(baseRequest(start, end), in)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Status != http.StatusConflict {
		t.Errorf("want 409, got %d", err.Status)
	}
}

func TestValidate_PendingCapReached(t *testing.T) {
	start := monday09()
	end := start.Add(2 * time.Hour)
	now := start.Add(-24 * time.Hour)

	in := baseInputs(now)
	in.Config.MaxPendingCap = 3
	in.PendingCount = 3

	_, err := booking.Validate(baseRequest(start, end), in)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Message != "pending booking limit reached; confirm or cancel existing bookings first" {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestValidate_WeekendSurcharge(t *testing.T) {
	// 2030-01-12 is a Saturday.
	start := time.Date(2030, 1, 12, 9, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	now := start.Add(-24 * time.Hour)

	in := baseInputs(now)
	in.Space.WeekendSurchargePct = 20

	proposal, err := booking.Validate(baseRequest(start, end), in)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	// 2h × 500 = 1000 base; +20% = 1200
	if proposal.TotalPrice != 1200 {
		t.Errorf("TotalPrice: want 1200, got %d", proposal.TotalPrice)
	}
}

func TestValidate_MinBillableHours(t *testing.T) {
	// Space with 120-min minimum; booking is exactly 60 min.
	// calculatePrice rounds up to minBillableHours (2h) → 2 × 500 = 1000.
	start := monday09()
	end := start.Add(time.Hour) // 1h < MinMinutes=120? No, MinMinutes=60 by default.
	// Override MinMinutes to 120 so the 1h booking hits the billable floor.
	now := start.Add(-24 * time.Hour)

	in := baseInputs(now)
	in.Space.MinMinutes = 60 // min booking is 1h; billing min is also 1h

	proposal, err := booking.Validate(baseRequest(start, end), in)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	// 1h × 500 = 500; min billable = ceil(60/60) = 1h → 500
	if proposal.TotalPrice != 500 {
		t.Errorf("TotalPrice: want 500, got %d", proposal.TotalPrice)
	}
}

// --- IsValidTransition ---

func TestIsValidTransition(t *testing.T) {
	cases := []struct {
		from store.BookingStatus
		to   store.BookingStatus
		want bool
	}{
		{store.BookingStatusPending, store.BookingStatusAwaitingPayment, true},
		{store.BookingStatusPending, store.BookingStatusCancelled, true},
		{store.BookingStatusPending, store.BookingStatusConfirmed, false},
		{store.BookingStatusAwaitingPayment, store.BookingStatusPaymentReview, true},
		{store.BookingStatusAwaitingPayment, store.BookingStatusCancelled, true},
		{store.BookingStatusAwaitingPayment, store.BookingStatusConfirmed, false},
		{store.BookingStatusPaymentReview, store.BookingStatusConfirmed, true},
		{store.BookingStatusPaymentReview, store.BookingStatusAwaitingPayment, true},
		{store.BookingStatusPaymentReview, store.BookingStatusCancelled, true},
		{store.BookingStatusPaymentReview, store.BookingStatusPending, false},
		{store.BookingStatusConfirmed, store.BookingStatusCancelled, true},
		{store.BookingStatusConfirmed, store.BookingStatusPending, false},
		{store.BookingStatusCompleted, store.BookingStatusCancelled, false},
	}
	for _, c := range cases {
		got := booking.IsValidTransition(c.from, c.to)
		if got != c.want {
			t.Errorf("IsValidTransition(%s→%s): want %v, got %v", c.from, c.to, c.want, got)
		}
	}
}
