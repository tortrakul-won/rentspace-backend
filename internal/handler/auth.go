package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

type AuthHandler struct {
	q         store.Store
	jwtSecret string
}

// NewAuthHandler stores the database queries and JWT secret so all auth methods can use them.
func NewAuthHandler(q store.Store, jwtSecret string) *AuthHandler {
	return &AuthHandler{q: q, jwtSecret: jwtSecret}
}

// Register creates a brand new account.
// Validates required fields, hashes the password with bcrypt, inserts a users row,
// inserts a profiles row with the chosen role, and returns a signed JWT + user + profile.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Email == "" || body.Password == "" || body.ProfileRole == "" ||
		body.ProfileName == "" || body.LegalNameTh == "" || body.Phone == "" ||
		body.AddressLine1 == "" || body.Subdistrict == "" || body.District == "" ||
		body.Province == "" || body.PostalCode == "" {
		Error(w, http.StatusBadRequest, "email, password, profile_role, profile_name, legal_name_th, phone, address_line1, subdistrict, district, province and postal_code are required")
		return
	}
	if body.ProfileRole != "owner" && body.ProfileRole != "renter" {
		Error(w, http.StatusBadRequest, "profile_role must be owner or renter")
		return
	}
	if body.ProfileRole == "owner" && body.TaxID == "" {
		Error(w, http.StatusBadRequest, "tax_id is required for owner profiles")
		return
	}
	if len(body.Password) < 8 {
		Error(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	// bcrypt silently truncates at 72 bytes — enforce the limit explicitly to prevent
	// two different long passwords hashing identically, and block CPU-exhaustion via huge inputs
	if len([]byte(body.Password)) > 72 {
		Error(w, http.StatusBadRequest, "password must be 72 characters or fewer")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	var user store.User
	var profile store.Profile
	if err := h.q.ExecTx(r.Context(), func(q store.Querier) error {
		var err error
		user, err = q.CreateUser(r.Context(), store.CreateUserParams{
			Email:        body.Email,
			PasswordHash: string(hash),
		})
		if err != nil {
			return err
		}
		profile, err = q.CreateProfile(r.Context(), store.CreateProfileParams{
			UserID:          user.ID,
			Role:            store.ProfileRole(body.ProfileRole),
			ProfileName:     body.ProfileName,
			LegalNameTh:     body.LegalNameTh,
			LegalNameEn:     body.LegalNameEn,
			Phone:           body.Phone,
			AddressLine1:    body.AddressLine1,
			Subdistrict:     body.Subdistrict,
			District:        body.District,
			Province:        body.Province,
			PostalCode:      body.PostalCode,
			BranchNumber:    body.BranchNumber,
			TaxID:           sql.NullString{String: body.TaxID, Valid: body.TaxID != ""},
			IsJuristic:      body.IsJuristic,
			IsVatRegistered: body.IsVatRegistered,
			LineID:          sql.NullString{String: body.LineID, Valid: body.LineID != ""},
		})
		return err
	}); err != nil {
		if isUniqueViolation(err) {
			Error(w, http.StatusConflict, "email already in use")
			return
		}
		ServerError(w, r, err)
		return
	}

	token, err := h.signToken(user.ID, profile.ID, string(profile.Role), user.IsAdmin)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	JSON(w, http.StatusCreated, AuthResponse{
		Token:   token,
		User:    toUserResponse(user),
		Profile: toProfileResponse(profile),
	})
}

// Login authenticates an existing user.
// Looks up the user by email, compares the password against the bcrypt hash,
// loads all active profiles, and returns a JWT (first profile active) + user + all profiles.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.q.GetUserByEmail(r.Context(), body.Email)
	if err != nil {
		Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	profiles, err := h.q.GetProfilesByUserID(r.Context(), user.ID)
	if err != nil || len(profiles) == 0 {
		ServerError(w, r, err)
		return
	}

	active := profiles[0]
	token, err := h.signToken(user.ID, active.ID, string(active.Role), user.IsAdmin)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	profileResponses := make([]ProfileResponse, len(profiles))
	for i, p := range profiles {
		profileResponses[i] = toProfileResponse(p)
	}

	JSON(w, http.StatusOK, LoginResponse{
		Token:           token,
		User:            toUserResponse(user),
		Profiles:        profileResponses,
		ActiveProfileID: active.ID.String(),
	})
}

// SwitchProfile swaps the active profile in the JWT without re-logging in.
// Verifies the requested profile exists and belongs to the current user,
// then returns a fresh JWT with the new profile embedded.
func (h *AuthHandler) SwitchProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())

	var body SwitchProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	profileID, err := uuid.Parse(body.ProfileID)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid profile_id")
		return
	}

	profile, err := h.q.GetProfileByID(r.Context(), profileID)
	if err != nil {
		Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if profile.UserID != claims.UserID {
		Error(w, http.StatusForbidden, "profile does not belong to current user")
		return
	}

	token, err := h.signToken(claims.UserID, profile.ID, string(profile.Role), claims.IsAdmin)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	JSON(w, http.StatusOK, SwitchProfileResponse{
		Token:   token,
		Profile: toProfileResponse(profile),
	})
}

// AddProfile adds a second profile (owner or renter) to an existing account.
// The DB unique constraint on (user_id, role) rejects duplicate roles automatically.
func (h *AuthHandler) AddProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())

	var body AddProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Role != "owner" && body.Role != "renter" {
		Error(w, http.StatusBadRequest, "role must be owner or renter")
		return
	}
	if body.ProfileName == "" || body.LegalNameTh == "" || body.Phone == "" ||
		body.AddressLine1 == "" || body.Subdistrict == "" || body.District == "" ||
		body.Province == "" || body.PostalCode == "" {
		Error(w, http.StatusBadRequest, "profile_name, legal_name_th, phone, address_line1, subdistrict, district, province and postal_code are required")
		return
	}
	if body.Role == "owner" && body.TaxID == "" {
		Error(w, http.StatusBadRequest, "tax_id is required for owner profiles")
		return
	}

	profile, err := h.q.CreateProfile(r.Context(), store.CreateProfileParams{
		UserID:          claims.UserID,
		Role:            store.ProfileRole(body.Role),
		ProfileName:     body.ProfileName,
		LegalNameTh:     body.LegalNameTh,
		LegalNameEn:     body.LegalNameEn,
		Phone:           body.Phone,
		AddressLine1:    body.AddressLine1,
		Subdistrict:     body.Subdistrict,
		District:        body.District,
		Province:        body.Province,
		PostalCode:      body.PostalCode,
		BranchNumber:    body.BranchNumber,
		TaxID:           sql.NullString{String: body.TaxID, Valid: body.TaxID != ""},
		IsJuristic:      body.IsJuristic,
		IsVatRegistered: body.IsVatRegistered,
		LineID:          sql.NullString{String: body.LineID, Valid: body.LineID != ""},
	})
	if err != nil {
		Error(w, http.StatusConflict, "a profile for this role already exists")
		return
	}

	JSON(w, http.StatusCreated, toProfileResponse(profile))
}

// CurrentUser returns the authenticated user's full state:
// their account details, all active profiles, and which profile is currently active in the JWT.
func (h *AuthHandler) CurrentUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())

	user, err := h.q.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		Error(w, http.StatusNotFound, "user not found")
		return
	}
	profiles, err := h.q.GetProfilesByUserID(r.Context(), user.ID)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	profileResponses := make([]ProfileResponse, len(profiles))
	for i, p := range profiles {
		profileResponses[i] = toProfileResponse(p)
	}

	JSON(w, http.StatusOK, CurrentUserResponse{
		User:            toUserResponse(user),
		Profiles:        profileResponses,
		ActiveProfileID: claims.ProfileID.String(),
	})
}

// signToken builds a JWT with user_id, profile_id, role and a 24h expiry, signed with HS256.
func (h *AuthHandler) signToken(userID, profileID uuid.UUID, role string, isAdmin bool) (string, error) {
	claims := middleware.Claims{
		UserID:    userID,
		ProfileID: profileID,
		Role:      role,
		IsAdmin:   isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.jwtSecret))
}

// toUserResponse maps a store.User to the API response shape, stripping the password hash.
func toUserResponse(u store.User) UserResponse {
	return UserResponse{
		ID:        u.ID.String(),
		Email:     u.Email,
		IsAdmin:   u.IsAdmin,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}

// toProfileResponse maps a store.Profile to the API response shape.
func toProfileResponse(p store.Profile) ProfileResponse {
	return ProfileResponse{
		ID:              p.ID.String(),
		UserID:          p.UserID.String(),
		Role:            string(p.Role),
		ProfileName:     p.ProfileName,
		LegalNameTh:     p.LegalNameTh,
		LegalNameEn:     p.LegalNameEn,
		Phone:           p.Phone,
		AddressLine1:    p.AddressLine1,
		Subdistrict:     p.Subdistrict,
		District:        p.District,
		Province:        p.Province,
		PostalCode:      p.PostalCode,
		BranchNumber:    p.BranchNumber,
		TaxID:           p.TaxID.String,
		IsJuristic:      p.IsJuristic,
		IsVatRegistered: p.IsVatRegistered,
		LineID:          p.LineID.String,
		CreatedAt:       p.CreatedAt.Format(time.RFC3339),
	}
}

// UpdateProfile updates editable fields for the active profile.
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())

	var body struct {
		ProfileName     string `json:"profile_name"`
		LegalNameTh     string `json:"legal_name_th"`
		LegalNameEn     string `json:"legal_name_en"`
		Phone           string `json:"phone"`
		AddressLine1    string `json:"address_line1"`
		Subdistrict     string `json:"subdistrict"`
		District        string `json:"district"`
		Province        string `json:"province"`
		PostalCode      string `json:"postal_code"`
		BranchNumber    string `json:"branch_number"`
		TaxID           string `json:"tax_id"`
		IsJuristic      *bool  `json:"is_juristic"`
		IsVatRegistered *bool  `json:"is_vat_registered"`
		LineID          string `json:"line_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existing, err := h.q.GetProfileByID(r.Context(), claims.ProfileID)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	// Preserve existing values for omitted fields
	if body.ProfileName == "" {
		body.ProfileName = existing.ProfileName
	}
	if body.LegalNameTh == "" {
		body.LegalNameTh = existing.LegalNameTh
	}
	// LegalNameEn intentionally not preserved — empty string is valid (bilingual optional)
	if body.Phone == "" {
		body.Phone = existing.Phone
	}
	if body.AddressLine1 == "" {
		body.AddressLine1 = existing.AddressLine1
	}
	if body.Subdistrict == "" {
		body.Subdistrict = existing.Subdistrict
	}
	if body.District == "" {
		body.District = existing.District
	}
	if body.Province == "" {
		body.Province = existing.Province
	}
	if body.PostalCode == "" {
		body.PostalCode = existing.PostalCode
	}
	if body.BranchNumber == "" {
		body.BranchNumber = existing.BranchNumber
	}
	taxID := sql.NullString{String: body.TaxID, Valid: body.TaxID != ""}
	if !taxID.Valid {
		taxID = existing.TaxID
	}
	isJuristic := existing.IsJuristic
	if body.IsJuristic != nil {
		isJuristic = *body.IsJuristic
	}
	isVatRegistered := existing.IsVatRegistered
	if body.IsVatRegistered != nil {
		isVatRegistered = *body.IsVatRegistered
	}

	updated, err := h.q.UpdateProfile(r.Context(), store.UpdateProfileParams{
		ID:              claims.ProfileID,
		ProfileName:     body.ProfileName,
		LegalNameTh:     body.LegalNameTh,
		LegalNameEn:     body.LegalNameEn,
		Phone:           body.Phone,
		AddressLine1:    body.AddressLine1,
		Subdistrict:     body.Subdistrict,
		District:        body.District,
		Province:        body.Province,
		PostalCode:      body.PostalCode,
		BranchNumber:    body.BranchNumber,
		TaxID:           taxID,
		IsJuristic:      isJuristic,
		IsVatRegistered: isVatRegistered,
		LineID:          sql.NullString{String: body.LineID, Valid: body.LineID != ""},
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}

	JSON(w, http.StatusOK, toProfileResponse(updated))
}
