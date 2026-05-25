package handler

import (
	"encoding/json"
	"time"

	"rentspace/backend/internal/store"
)

// request / response types used for the API boundary
// handlers decode into these, then map to store.* types as needed

// --- Notifications ---

type NotificationResponse struct {
	ID           string          `json:"id"`
	ProfileID    string          `json:"profile_id"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload"`
	BookingID    *string         `json:"booking_id"`
	ReadAt       *string         `json:"read_at"`
	SupersededAt *string         `json:"superseded_at"`
	CreatedAt    string          `json:"created_at"`
}

func notificationToResponse(n store.Notification) NotificationResponse {
	r := NotificationResponse{
		ID:        n.ID.String(),
		ProfileID: n.ProfileID.String(),
		Type:      n.Type,
		Payload:   n.Payload,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
	}
	if n.BookingID.Valid {
		s := n.BookingID.UUID.String()
		r.BookingID = &s
	}
	if n.ReadAt.Valid {
		s := n.ReadAt.Time.Format(time.RFC3339)
		r.ReadAt = &s
	}
	if n.SupersededAt.Valid {
		s := n.SupersededAt.Time.Format(time.RFC3339)
		r.SupersededAt = &s
	}
	return r
}

// --- Auth ---

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	FullName    string `json:"full_name"`
	Phone       string `json:"phone,omitempty"`
	ProfileRole string `json:"profile_role"` // "owner" or "renter"
	DisplayName string `json:"display_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SwitchProfileRequest struct {
	ProfileID string `json:"profile_id"`
}

type AddProfileRequest struct {
	Role        string `json:"role"` // "owner" or "renter"
	DisplayName string `json:"display_name"`
}

type AuthResponse struct {
	Token   string          `json:"token"`
	User    UserResponse    `json:"user"`
	Profile ProfileResponse `json:"profile"`
}

type LoginResponse struct {
	Token           string            `json:"token"`
	User            UserResponse      `json:"user"`
	Profiles        []ProfileResponse `json:"profiles"`
	ActiveProfileID string            `json:"active_profile_id"`
}

type SwitchProfileResponse struct {
	Token   string          `json:"token"`
	Profile ProfileResponse `json:"profile"`
}

type CurrentUserResponse struct {
	User            UserResponse      `json:"user"`
	Profiles        []ProfileResponse `json:"profiles"`
	ActiveProfileID string            `json:"active_profile_id"`
}

type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FullName  string `json:"full_name"`
	Phone     string `json:"phone,omitempty"`
	IsAdmin   bool   `json:"is_admin"`
	CreatedAt string `json:"created_at"`
}

type ProfileResponse struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	Role            string `json:"role"`
	DisplayName     string `json:"display_name"`
	TaxID           string `json:"tax_id,omitempty"`
	IsJuristic      bool   `json:"is_juristic"`
	IsVatRegistered bool   `json:"is_vat_registered"`
	CreatedAt       string `json:"created_at"`
}

// --- Spaces ---

type CreateSpaceRequest struct {
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Location            string   `json:"location"`
	Category            string   `json:"category"`
	Images              []string `json:"images"`
	HourlyRate          int32    `json:"hourly_rate"`
	DailyRate           int32    `json:"daily_rate"`
	MinMinutes          int32    `json:"min_minutes"`
	Capacity            int32    `json:"capacity"`
	Amenities           []string `json:"amenities"`
	WeekendSurchargePct int32    `json:"weekend_surcharge_pct"`
}

type SpaceResponse struct {
	ID                  string   `json:"id"`
	OwnerID             string   `json:"owner_id"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Location            string   `json:"location"`
	Category            string   `json:"category"`
	Images              []string `json:"images"`
	HourlyRate          int32    `json:"hourly_rate"`
	DailyRate           int32    `json:"daily_rate"`
	MinMinutes          int32    `json:"min_minutes"`
	Capacity            int32    `json:"capacity"`
	Amenities           []string `json:"amenities"`
	WeekendSurchargePct int32    `json:"weekend_surcharge_pct"`
	IsActive            bool     `json:"is_active"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}

type AvailabilitySlot struct {
	DayOfWeek int    `json:"day_of_week"`
	OpenTime  string `json:"open_time"`
	CloseTime string `json:"close_time"`
}

type SetAvailabilityRequest struct {
	Schedule []AvailabilitySlot `json:"schedule"`
}

type BlockedRange struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // "booking" | "block"
}

type DayAvailability struct {
	Open          bool           `json:"open"`
	OpenTime      string         `json:"open_time,omitempty"`
	CloseTime     string         `json:"close_time,omitempty"`
	BlockedRanges []BlockedRange `json:"blocked_ranges"`
}

// --- Bookings ---

type CreateBookingRequest struct {
	SpaceID   string `json:"space_id"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Headcount *int   `json:"headcount,omitempty"`
	Notes     string `json:"notes,omitempty"`
	// total_price and platform_fee are server-calculated — not accepted from the client
}

type BookingResponse struct {
	ID               string  `json:"id"`
	SpaceID          string  `json:"space_id"`
	RenterID         string  `json:"renter_id"`
	StartTime        string  `json:"start_time"`
	EndTime          string  `json:"end_time"`
	TotalPrice       int32   `json:"total_price"`
	PlatformFee      int32   `json:"platform_fee"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	RefundStatus     *string `json:"refund_status"`
	ProcessExpiresAt *string `json:"process_expires_at"`
	SlipUrl          *string `json:"slip_url"`
}

// --- Shared ---

type ErrorResponse struct {
	Error string `json:"error"`
}

type Page[T any] struct {
	Data    []T   `json:"data"`
	Total   int64 `json:"total"`
	Page    int32 `json:"page"`
	Limit   int32 `json:"limit"`
	HasMore bool  `json:"has_more"`
}
