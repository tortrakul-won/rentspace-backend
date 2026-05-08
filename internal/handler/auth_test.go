package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

func newAuthHandler(q *mockStore) *AuthHandler {
	return NewAuthHandler(q, testSecret)
}

func decodeJSON(t *testing.T, body *bytes.Buffer, dst any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(dst); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	q := &mockStore{
		createUser:    func(_ context.Context, _ store.CreateUserParams) (store.User, error) { return stubUser(), nil },
		createProfile: func(_ context.Context, _ store.CreateProfileParams) (store.Profile, error) { return stubProfile(store.ProfileRoleRenter), nil },
	}
	body, _ := json.Marshal(RegisterRequest{
		Email: "test@example.com", Password: "password",
		FullName: "Test User", ProfileRole: "renter", DisplayName: "Personal",
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))

	newAuthHandler(q).Register(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp AuthResponse
	decodeJSON(t, w.Body, &resp)
	if resp.Token == "" {
		t.Error("expected token in response")
	}
	if resp.User.Email != "test@example.com" {
		t.Errorf("unexpected email: %s", resp.User.Email)
	}
}

func TestRegister_MissingFields(t *testing.T) {
	cases := []RegisterRequest{
		{Password: "pw", FullName: "n", ProfileRole: "renter", DisplayName: "p"},   // no email
		{Email: "a@b.com", FullName: "n", ProfileRole: "renter", DisplayName: "p"}, // no password
		{Email: "a@b.com", Password: "pw", ProfileRole: "renter", DisplayName: "p"}, // no full_name
		{Email: "a@b.com", Password: "pw", FullName: "n", DisplayName: "p"},         // no profile_role
		{Email: "a@b.com", Password: "pw", FullName: "n", ProfileRole: "renter"},    // no display_name
	}
	for _, tc := range cases {
		body, _ := json.Marshal(tc)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
		newAuthHandler(&mockStore{}).Register(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for %+v, got %d", tc, w.Code)
		}
	}
}

func TestRegister_InvalidRole(t *testing.T) {
	body, _ := json.Marshal(RegisterRequest{
		Email: "a@b.com", Password: "pw", FullName: "n", ProfileRole: "admin", DisplayName: "p",
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	newAuthHandler(&mockStore{}).Register(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	q := &mockStore{
		createUser: func(_ context.Context, _ store.CreateUserParams) (store.User, error) {
			return store.User{}, errors.New("duplicate")
		},
	}
	body, _ := json.Marshal(RegisterRequest{
		Email: "dup@example.com", Password: "password", FullName: "n", ProfileRole: "renter", DisplayName: "p",
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	newAuthHandler(q).Register(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	q := &mockStore{
		getUserByEmail:      func(_ context.Context, _ string) (store.User, error) { return stubUser(), nil },
		getProfilesByUserID: func(_ context.Context, _ uuid.UUID) ([]store.Profile, error) {
			return []store.Profile{stubProfile(store.ProfileRoleRenter)}, nil
		},
	}
	body, _ := json.Marshal(LoginRequest{Email: "test@example.com", Password: "password"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))

	newAuthHandler(q).Login(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp LoginResponse
	decodeJSON(t, w.Body, &resp)
	if resp.Token == "" {
		t.Error("expected token in response")
	}
	if len(resp.Profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(resp.Profiles))
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	q := &mockStore{
		getUserByEmail: func(_ context.Context, _ string) (store.User, error) { return stubUser(), nil },
	}
	body, _ := json.Marshal(LoginRequest{Email: "test@example.com", Password: "wrong"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))

	newAuthHandler(q).Login(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	q := &mockStore{
		getUserByEmail: func(_ context.Context, _ string) (store.User, error) {
			return store.User{}, errors.New("not found")
		},
	}
	body, _ := json.Marshal(LoginRequest{Email: "ghost@example.com", Password: "pw"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))

	newAuthHandler(q).Login(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- SwitchProfile ---

func TestSwitchProfile_Success(t *testing.T) {
	profile := stubProfile(store.ProfileRoleOwner)
	q := &mockStore{
		getProfileByID: func(_ context.Context, _ uuid.UUID) (store.Profile, error) { return profile, nil },
	}
	body, _ := json.Marshal(SwitchProfileRequest{ProfileID: testProfileID.String()})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/switch-profile", bytes.NewReader(body))
	r = r.WithContext(middleware.ContextWithClaims(r.Context(), &middleware.Claims{
		UserID:    testUserID,
		ProfileID: testProfileID,
		Role:      "renter",
	}))

	newAuthHandler(q).SwitchProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp SwitchProfileResponse
	decodeJSON(t, w.Body, &resp)
	if resp.Token == "" {
		t.Error("expected token in response")
	}
}

func TestSwitchProfile_WrongUser(t *testing.T) {
	otherProfile := stubProfile(store.ProfileRoleOwner)
	otherProfile.UserID = uuid.New() // belongs to someone else
	q := &mockStore{
		getProfileByID: func(_ context.Context, _ uuid.UUID) (store.Profile, error) { return otherProfile, nil },
	}
	body, _ := json.Marshal(SwitchProfileRequest{ProfileID: testProfileID.String()})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/switch-profile", bytes.NewReader(body))
	r = r.WithContext(middleware.ContextWithClaims(r.Context(), &middleware.Claims{
		UserID:    testUserID,
		ProfileID: testProfileID,
		Role:      "renter",
	}))

	newAuthHandler(q).SwitchProfile(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// --- AddProfile ---

func TestAddProfile_Success(t *testing.T) {
	ownerProfile := stubProfile(store.ProfileRoleOwner)
	q := &mockStore{
		createProfile: func(_ context.Context, _ store.CreateProfileParams) (store.Profile, error) { return ownerProfile, nil },
	}
	body, _ := json.Marshal(AddProfileRequest{Role: "owner", DisplayName: "My Property"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/profiles", bytes.NewReader(body))
	r = r.WithContext(middleware.ContextWithClaims(r.Context(), &middleware.Claims{
		UserID: testUserID, ProfileID: testProfileID, Role: "renter",
	}))

	newAuthHandler(q).AddProfile(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddProfile_DuplicateRole(t *testing.T) {
	q := &mockStore{
		createProfile: func(_ context.Context, _ store.CreateProfileParams) (store.Profile, error) {
			return store.Profile{}, errors.New("unique constraint")
		},
	}
	body, _ := json.Marshal(AddProfileRequest{Role: "renter", DisplayName: "Another"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/auth/profiles", bytes.NewReader(body))
	r = r.WithContext(middleware.ContextWithClaims(r.Context(), &middleware.Claims{
		UserID: testUserID, ProfileID: testProfileID, Role: "renter",
	}))

	newAuthHandler(q).AddProfile(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

// --- CurrentUser ---

func TestCurrentUser_Success(t *testing.T) {
	q := &mockStore{
		getUserByID: func(_ context.Context, _ uuid.UUID) (store.User, error) { return stubUser(), nil },
		getProfilesByUserID: func(_ context.Context, _ uuid.UUID) ([]store.Profile, error) {
			return []store.Profile{stubProfile(store.ProfileRoleRenter)}, nil
		},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	r = r.WithContext(middleware.ContextWithClaims(r.Context(), &middleware.Claims{
		UserID: testUserID, ProfileID: testProfileID, Role: "renter",
	}))

	newAuthHandler(q).CurrentUser(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp CurrentUserResponse
	decodeJSON(t, w.Body, &resp)
	if resp.User.Email != "test@example.com" {
		t.Errorf("unexpected email: %s", resp.User.Email)
	}
	if resp.ActiveProfileID != testProfileID.String() {
		t.Errorf("unexpected active profile: %s", resp.ActiveProfileID)
	}
}
