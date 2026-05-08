package handler

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"rentspace/backend/internal/store"
)

// pgUniqueErr returns a Postgres unique-constraint error (23505) for use in tests.
func pgUniqueErr() error {
	return &pgconn.PgError{Code: "23505"}
}

// mockStore implements store.Store for use in handler tests.
// Each field is a function so individual tests can override only what they need.
type mockStore struct {
	execTx func(ctx context.Context, fn func(store.Querier) error) error
	createUser               func(ctx context.Context, arg store.CreateUserParams) (store.User, error)
	getUserByEmail           func(ctx context.Context, email string) (store.User, error)
	getUserByID              func(ctx context.Context, id uuid.UUID) (store.User, error)
	createProfile            func(ctx context.Context, arg store.CreateProfileParams) (store.Profile, error)
	getProfilesByUserID      func(ctx context.Context, userID uuid.UUID) ([]store.Profile, error)
	getProfileByID           func(ctx context.Context, id uuid.UUID) (store.Profile, error)
	getProfileByUserAndRole  func(ctx context.Context, arg store.GetProfileByUserAndRoleParams) (store.Profile, error)
	createSpace              func(ctx context.Context, arg store.CreateSpaceParams) (store.Space, error)
	getSpaceByID             func(ctx context.Context, id uuid.UUID) (store.Space, error)
	listSpaces               func(ctx context.Context) ([]store.Space, error)
	listSpacesByCategory     func(ctx context.Context, category store.SpaceCategory) ([]store.Space, error)
	listSpacesByOwner        func(ctx context.Context, ownerID uuid.UUID) ([]store.Space, error)
	setSpaceActive           func(ctx context.Context, arg store.SetSpaceActiveParams) (store.Space, error)
	updateSpace              func(ctx context.Context, arg store.UpdateSpaceParams) (store.Space, error)
	createBooking            func(ctx context.Context, arg store.CreateBookingParams) (store.Booking, error)
	getBookingByID           func(ctx context.Context, id uuid.UUID) (store.Booking, error)
	listBookingsByRenter     func(ctx context.Context, renterID uuid.UUID) ([]store.Booking, error)
	listBookingsBySpace      func(ctx context.Context, spaceID uuid.UUID) ([]store.Booking, error)
	updateBookingStatus      func(ctx context.Context, arg store.UpdateBookingStatusParams) (store.Booking, error)
	checkOverlappingBookings func(ctx context.Context, arg store.CheckOverlappingBookingsParams) (int64, error)
}

func (m *mockStore) ExecTx(ctx context.Context, fn func(store.Querier) error) error {
	if m.execTx != nil {
		return m.execTx(ctx, fn)
	}
	return fn(m) // no real transaction in tests; pass the mock as the querier
}

func (m *mockStore) CreateUser(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
	if m.createUser != nil {
		return m.createUser(ctx, arg)
	}
	return store.User{}, errors.New("not implemented")
}
func (m *mockStore) GetUserByEmail(ctx context.Context, email string) (store.User, error) {
	if m.getUserByEmail != nil {
		return m.getUserByEmail(ctx, email)
	}
	return store.User{}, errors.New("not implemented")
}
func (m *mockStore) GetUserByID(ctx context.Context, id uuid.UUID) (store.User, error) {
	if m.getUserByID != nil {
		return m.getUserByID(ctx, id)
	}
	return store.User{}, errors.New("not implemented")
}
func (m *mockStore) CreateProfile(ctx context.Context, arg store.CreateProfileParams) (store.Profile, error) {
	if m.createProfile != nil {
		return m.createProfile(ctx, arg)
	}
	return store.Profile{}, errors.New("not implemented")
}
func (m *mockStore) GetProfilesByUserID(ctx context.Context, userID uuid.UUID) ([]store.Profile, error) {
	if m.getProfilesByUserID != nil {
		return m.getProfilesByUserID(ctx, userID)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) GetProfileByID(ctx context.Context, id uuid.UUID) (store.Profile, error) {
	if m.getProfileByID != nil {
		return m.getProfileByID(ctx, id)
	}
	return store.Profile{}, errors.New("not implemented")
}
func (m *mockStore) GetProfileByUserAndRole(ctx context.Context, arg store.GetProfileByUserAndRoleParams) (store.Profile, error) {
	if m.getProfileByUserAndRole != nil {
		return m.getProfileByUserAndRole(ctx, arg)
	}
	return store.Profile{}, errors.New("not implemented")
}
func (m *mockStore) CreateSpace(ctx context.Context, arg store.CreateSpaceParams) (store.Space, error) {
	if m.createSpace != nil {
		return m.createSpace(ctx, arg)
	}
	return store.Space{}, errors.New("not implemented")
}
func (m *mockStore) GetSpaceByID(ctx context.Context, id uuid.UUID) (store.Space, error) {
	if m.getSpaceByID != nil {
		return m.getSpaceByID(ctx, id)
	}
	return store.Space{}, errors.New("not implemented")
}
func (m *mockStore) ListSpaces(ctx context.Context) ([]store.Space, error) {
	if m.listSpaces != nil {
		return m.listSpaces(ctx)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) ListSpacesByCategory(ctx context.Context, category store.SpaceCategory) ([]store.Space, error) {
	if m.listSpacesByCategory != nil {
		return m.listSpacesByCategory(ctx, category)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) ListSpacesByOwner(ctx context.Context, ownerID uuid.UUID) ([]store.Space, error) {
	if m.listSpacesByOwner != nil {
		return m.listSpacesByOwner(ctx, ownerID)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) SetSpaceActive(ctx context.Context, arg store.SetSpaceActiveParams) (store.Space, error) {
	if m.setSpaceActive != nil {
		return m.setSpaceActive(ctx, arg)
	}
	return store.Space{}, errors.New("not implemented")
}
func (m *mockStore) UpdateSpace(ctx context.Context, arg store.UpdateSpaceParams) (store.Space, error) {
	if m.updateSpace != nil {
		return m.updateSpace(ctx, arg)
	}
	return store.Space{}, errors.New("not implemented")
}
func (m *mockStore) CreateBooking(ctx context.Context, arg store.CreateBookingParams) (store.Booking, error) {
	if m.createBooking != nil {
		return m.createBooking(ctx, arg)
	}
	return store.Booking{}, errors.New("not implemented")
}
func (m *mockStore) GetBookingByID(ctx context.Context, id uuid.UUID) (store.Booking, error) {
	if m.getBookingByID != nil {
		return m.getBookingByID(ctx, id)
	}
	return store.Booking{}, errors.New("not implemented")
}
func (m *mockStore) ListBookingsByRenter(ctx context.Context, renterID uuid.UUID) ([]store.Booking, error) {
	if m.listBookingsByRenter != nil {
		return m.listBookingsByRenter(ctx, renterID)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) ListBookingsBySpace(ctx context.Context, spaceID uuid.UUID) ([]store.Booking, error) {
	if m.listBookingsBySpace != nil {
		return m.listBookingsBySpace(ctx, spaceID)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) UpdateBookingStatus(ctx context.Context, arg store.UpdateBookingStatusParams) (store.Booking, error) {
	if m.updateBookingStatus != nil {
		return m.updateBookingStatus(ctx, arg)
	}
	return store.Booking{}, errors.New("not implemented")
}
func (m *mockStore) CheckOverlappingBookings(ctx context.Context, arg store.CheckOverlappingBookingsParams) (int64, error) {
	if m.checkOverlappingBookings != nil {
		return m.checkOverlappingBookings(ctx, arg)
	}
	return 0, errors.New("not implemented")
}

// fixtures

var (
	testUserID    = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testProfileID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testSecret    = "test-secret"
)

func stubUser() store.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	return store.User{
		ID:           testUserID,
		Email:        "test@example.com",
		PasswordHash: string(hash),
		FullName:     "Test User",
		Phone:        sql.NullString{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func stubProfile(role store.ProfileRole) store.Profile {
	return store.Profile{
		ID:          testProfileID,
		UserID:      testUserID,
		Role:        role,
		DisplayName: "Personal",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
