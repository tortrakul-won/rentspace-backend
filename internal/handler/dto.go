package handler

// request / response types used for swagger docs and API boundary
// handlers decode into these, then map to store.* types as needed

type CreateSpaceRequest struct {
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

type CreateBookingRequest struct {
	SpaceID     string `json:"space_id"`
	RenterID    string `json:"renter_id"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	TotalPrice  int32  `json:"total_price"`
	PlatformFee int32  `json:"platform_fee"`
	RenterType  string `json:"renter_type"`
	CompanyName string `json:"company_name,omitempty"`
	TaxID       string `json:"tax_id,omitempty"`
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
	RenterType  string `json:"renter_type"`
	CompanyName string `json:"company_name,omitempty"`
	TaxID       string `json:"tax_id,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
