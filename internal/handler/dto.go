package handler

// request / response types used for the API boundary
// handlers decode into these, then map to store.* types as needed

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
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	Category    string   `json:"category"`
	Images      []string `json:"images"`
	HourlyRate  int32    `json:"hourly_rate"`
	DailyRate   int32    `json:"daily_rate"`
	MinHours    int32    `json:"min_hours"`
	Capacity    int32    `json:"capacity"`
	Amenities   []string `json:"amenities"`
}

type SpaceResponse struct {
	ID          string   `json:"id"`
	OwnerID     string   `json:"owner_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	Category    string   `json:"category"`
	Images      []string `json:"images"`
	HourlyRate  int32    `json:"hourly_rate"`
	DailyRate   int32    `json:"daily_rate"`
	MinHours    int32    `json:"min_hours"`
	Capacity    int32    `json:"capacity"`
	Amenities   []string `json:"amenities"`
	IsActive    bool     `json:"is_active"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// --- Bookings ---

type CreateBookingRequest struct {
	SpaceID   string `json:"space_id"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	// total_price and platform_fee are server-calculated — not accepted from the client
}

type BookingResponse struct {
	ID          string `json:"id"`
	SpaceID     string `json:"space_id"`
	RenterID    string `json:"renter_id"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	TotalPrice  int32  `json:"total_price"`
	PlatformFee int32  `json:"platform_fee"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// --- Shared ---

type ErrorResponse struct {
	Error string `json:"error"`
}
