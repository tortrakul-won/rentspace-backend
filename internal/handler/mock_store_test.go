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
	updateSpace                  func(ctx context.Context, arg store.UpdateSpaceParams) (store.Space, error)
	deleteSpace                      func(ctx context.Context, arg store.DeleteSpaceParams) error
	getSpaceAvailability             func(ctx context.Context, spaceID uuid.UUID) ([]store.SpaceAvailability, error)
	upsertSpaceAvailability          func(ctx context.Context, arg store.UpsertSpaceAvailabilityParams) (store.SpaceAvailability, error)
	deleteSpaceAvailability          func(ctx context.Context, spaceID uuid.UUID) error
	listSpacesPaginated                          func(ctx context.Context, arg store.ListSpacesPaginatedParams) ([]store.Space, error)
	countSpaces                                  func(ctx context.Context) (int64, error)
	listSpacesPaginatedExcludeUser               func(ctx context.Context, arg store.ListSpacesPaginatedExcludeUserParams) ([]store.Space, error)
	countSpacesExcludeUser                       func(ctx context.Context, userID uuid.UUID) (int64, error)
	listSpacesByCategoryPaginated                func(ctx context.Context, arg store.ListSpacesByCategoryPaginatedParams) ([]store.Space, error)
	countSpacesByCategory                        func(ctx context.Context, category store.SpaceCategory) (int64, error)
	listSpacesByCategoryPaginatedExcludeUser     func(ctx context.Context, arg store.ListSpacesByCategoryPaginatedExcludeUserParams) ([]store.Space, error)
	countSpacesByCategoryExcludeUser             func(ctx context.Context, arg store.CountSpacesByCategoryExcludeUserParams) (int64, error)
	listSpacesByOwnerPaginated       func(ctx context.Context, arg store.ListSpacesByOwnerPaginatedParams) ([]store.Space, error)
	countSpacesByOwner               func(ctx context.Context, ownerID uuid.UUID) (int64, error)
	listBookingsBySpacePaginated     func(ctx context.Context, arg store.ListBookingsBySpacePaginatedParams) ([]store.Booking, error)
	countBookingsBySpace             func(ctx context.Context, spaceID uuid.UUID) (int64, error)
	createBooking            func(ctx context.Context, arg store.CreateBookingParams) (store.Booking, error)
	getBookingByID           func(ctx context.Context, id uuid.UUID) (store.Booking, error)
	listBookingsByOwner      func(ctx context.Context, ownerID uuid.UUID) ([]store.Booking, error)
	listBookingsByRenter     func(ctx context.Context, renterID uuid.UUID) ([]store.Booking, error)
	listBookingsBySpace      func(ctx context.Context, spaceID uuid.UUID) ([]store.Booking, error)
	updateBookingStatus      func(ctx context.Context, arg store.UpdateBookingStatusParams) (store.Booking, error)
	checkOverlappingBookings              func(ctx context.Context, arg store.CheckOverlappingBookingsParams) (int64, error)
	bulkCompleteConfirmedBookings         func(ctx context.Context) ([]store.Booking, error)
	bulkExpirePendingBookings             func(ctx context.Context) ([]store.Booking, error)
	cancelOverlappingPendingBookings      func(ctx context.Context, arg store.CancelOverlappingPendingBookingsParams) ([]store.Booking, error)
	countActiveBookingsByRenterForSpace   func(ctx context.Context, arg store.CountActiveBookingsByRenterForSpaceParams) (int64, error)
	countPendingBookingsByRenter          func(ctx context.Context, renterID uuid.UUID) (int64, error)
	createSpaceBlock                      func(ctx context.Context, arg store.CreateSpaceBlockParams) (store.SpaceBlock, error)
	listSpaceBlocksBySpace                func(ctx context.Context, spaceID uuid.UUID) ([]store.SpaceBlock, error)
	getSpaceBlocksInRange                 func(ctx context.Context, arg store.GetSpaceBlocksInRangeParams) ([]store.SpaceBlock, error)
	checkOverlappingSpaceBlocks           func(ctx context.Context, arg store.CheckOverlappingSpaceBlocksParams) (int64, error)
	deleteSpaceBlock                      func(ctx context.Context, arg store.DeleteSpaceBlockParams) error
	getSystemConfig                       func(ctx context.Context, key string) (string, error)
	setSystemConfig                       func(ctx context.Context, arg store.SetSystemConfigParams) error
	createNotification                    func(ctx context.Context, arg store.CreateNotificationParams) (store.Notification, error)
	listNotificationsByProfile            func(ctx context.Context, arg store.ListNotificationsByProfileParams) ([]store.Notification, error)
	countUnreadNotifications              func(ctx context.Context, profileID uuid.UUID) (int64, error)
	markNotificationRead                  func(ctx context.Context, arg store.MarkNotificationReadParams) error
	markAllNotificationsRead              func(ctx context.Context, profileID uuid.UUID) error
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
func (m *mockStore) DeleteSpace(ctx context.Context, arg store.DeleteSpaceParams) error {
	if m.deleteSpace != nil {
		return m.deleteSpace(ctx, arg)
	}
	return errors.New("not implemented")
}
func (m *mockStore) GetSpaceAvailability(ctx context.Context, spaceID uuid.UUID) ([]store.SpaceAvailability, error) {
	if m.getSpaceAvailability != nil {
		return m.getSpaceAvailability(ctx, spaceID)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) UpsertSpaceAvailability(ctx context.Context, arg store.UpsertSpaceAvailabilityParams) (store.SpaceAvailability, error) {
	if m.upsertSpaceAvailability != nil {
		return m.upsertSpaceAvailability(ctx, arg)
	}
	return store.SpaceAvailability{}, errors.New("not implemented")
}
func (m *mockStore) DeleteSpaceAvailability(ctx context.Context, spaceID uuid.UUID) error {
	if m.deleteSpaceAvailability != nil {
		return m.deleteSpaceAvailability(ctx, spaceID)
	}
	return errors.New("not implemented")
}
func (m *mockStore) ListSpacesPaginated(ctx context.Context, arg store.ListSpacesPaginatedParams) ([]store.Space, error) {
	if m.listSpacesPaginated != nil {
		return m.listSpacesPaginated(ctx, arg)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) CountSpaces(ctx context.Context) (int64, error) {
	if m.countSpaces != nil {
		return m.countSpaces(ctx)
	}
	return 0, errors.New("not implemented")
}
func (m *mockStore) ListSpacesPaginatedExcludeUser(ctx context.Context, arg store.ListSpacesPaginatedExcludeUserParams) ([]store.Space, error) {
	if m.listSpacesPaginatedExcludeUser != nil {
		return m.listSpacesPaginatedExcludeUser(ctx, arg)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) CountSpacesExcludeUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	if m.countSpacesExcludeUser != nil {
		return m.countSpacesExcludeUser(ctx, userID)
	}
	return 0, errors.New("not implemented")
}
func (m *mockStore) ListSpacesByCategoryPaginatedExcludeUser(ctx context.Context, arg store.ListSpacesByCategoryPaginatedExcludeUserParams) ([]store.Space, error) {
	if m.listSpacesByCategoryPaginatedExcludeUser != nil {
		return m.listSpacesByCategoryPaginatedExcludeUser(ctx, arg)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) CountSpacesByCategoryExcludeUser(ctx context.Context, arg store.CountSpacesByCategoryExcludeUserParams) (int64, error) {
	if m.countSpacesByCategoryExcludeUser != nil {
		return m.countSpacesByCategoryExcludeUser(ctx, arg)
	}
	return 0, errors.New("not implemented")
}
func (m *mockStore) ListSpacesByCategoryPaginated(ctx context.Context, arg store.ListSpacesByCategoryPaginatedParams) ([]store.Space, error) {
	if m.listSpacesByCategoryPaginated != nil {
		return m.listSpacesByCategoryPaginated(ctx, arg)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) CountSpacesByCategory(ctx context.Context, category store.SpaceCategory) (int64, error) {
	if m.countSpacesByCategory != nil {
		return m.countSpacesByCategory(ctx, category)
	}
	return 0, errors.New("not implemented")
}
func (m *mockStore) ListSpacesByOwnerPaginated(ctx context.Context, arg store.ListSpacesByOwnerPaginatedParams) ([]store.Space, error) {
	if m.listSpacesByOwnerPaginated != nil {
		return m.listSpacesByOwnerPaginated(ctx, arg)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) CountSpacesByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	if m.countSpacesByOwner != nil {
		return m.countSpacesByOwner(ctx, ownerID)
	}
	return 0, errors.New("not implemented")
}
func (m *mockStore) ListBookingsBySpacePaginated(ctx context.Context, arg store.ListBookingsBySpacePaginatedParams) ([]store.Booking, error) {
	if m.listBookingsBySpacePaginated != nil {
		return m.listBookingsBySpacePaginated(ctx, arg)
	}
	return nil, errors.New("not implemented")
}
func (m *mockStore) CountBookingsBySpace(ctx context.Context, spaceID uuid.UUID) (int64, error) {
	if m.countBookingsBySpace != nil {
		return m.countBookingsBySpace(ctx, spaceID)
	}
	return 0, errors.New("not implemented")
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
func (m *mockStore) ListBookingsByOwner(ctx context.Context, ownerID uuid.UUID) ([]store.Booking, error) {
	if m.listBookingsByOwner != nil {
		return m.listBookingsByOwner(ctx, ownerID)
	}
	return nil, errors.New("not implemented")
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
func (m *mockStore) BulkCompleteConfirmedBookings(ctx context.Context) ([]store.Booking, error) {
	if m.bulkCompleteConfirmedBookings != nil {
		return m.bulkCompleteConfirmedBookings(ctx)
	}
	return nil, nil
}
func (m *mockStore) BulkExpirePendingBookings(ctx context.Context) ([]store.Booking, error) {
	if m.bulkExpirePendingBookings != nil {
		return m.bulkExpirePendingBookings(ctx)
	}
	return nil, nil
}
func (m *mockStore) CancelOverlappingPendingBookings(ctx context.Context, arg store.CancelOverlappingPendingBookingsParams) ([]store.Booking, error) {
	if m.cancelOverlappingPendingBookings != nil {
		return m.cancelOverlappingPendingBookings(ctx, arg)
	}
	return nil, nil
}
func (m *mockStore) CountActiveBookingsByRenterForSpace(ctx context.Context, arg store.CountActiveBookingsByRenterForSpaceParams) (int64, error) {
	if m.countActiveBookingsByRenterForSpace != nil {
		return m.countActiveBookingsByRenterForSpace(ctx, arg)
	}
	return 0, nil
}
func (m *mockStore) CountPendingBookingsByRenter(ctx context.Context, renterID uuid.UUID) (int64, error) {
	if m.countPendingBookingsByRenter != nil {
		return m.countPendingBookingsByRenter(ctx, renterID)
	}
	return 0, nil
}
func (m *mockStore) CreateSpaceBlock(ctx context.Context, arg store.CreateSpaceBlockParams) (store.SpaceBlock, error) {
	if m.createSpaceBlock != nil {
		return m.createSpaceBlock(ctx, arg)
	}
	return store.SpaceBlock{}, errors.New("not implemented")
}
func (m *mockStore) ListSpaceBlocksBySpace(ctx context.Context, spaceID uuid.UUID) ([]store.SpaceBlock, error) {
	if m.listSpaceBlocksBySpace != nil {
		return m.listSpaceBlocksBySpace(ctx, spaceID)
	}
	return nil, nil
}
func (m *mockStore) GetSpaceBlocksInRange(ctx context.Context, arg store.GetSpaceBlocksInRangeParams) ([]store.SpaceBlock, error) {
	if m.getSpaceBlocksInRange != nil {
		return m.getSpaceBlocksInRange(ctx, arg)
	}
	return nil, nil
}
func (m *mockStore) CheckOverlappingSpaceBlocks(ctx context.Context, arg store.CheckOverlappingSpaceBlocksParams) (int64, error) {
	if m.checkOverlappingSpaceBlocks != nil {
		return m.checkOverlappingSpaceBlocks(ctx, arg)
	}
	return 0, nil
}
func (m *mockStore) DeleteSpaceBlock(ctx context.Context, arg store.DeleteSpaceBlockParams) error {
	if m.deleteSpaceBlock != nil {
		return m.deleteSpaceBlock(ctx, arg)
	}
	return errors.New("not implemented")
}
func (m *mockStore) GetSystemConfig(ctx context.Context, key string) (string, error) {
	if m.getSystemConfig != nil {
		return m.getSystemConfig(ctx, key)
	}
	return "", errors.New("not implemented")
}
func (m *mockStore) SetSystemConfig(ctx context.Context, arg store.SetSystemConfigParams) error {
	if m.setSystemConfig != nil {
		return m.setSystemConfig(ctx, arg)
	}
	return errors.New("not implemented")
}
func (m *mockStore) CreateNotification(ctx context.Context, arg store.CreateNotificationParams) (store.Notification, error) {
	if m.createNotification != nil {
		return m.createNotification(ctx, arg)
	}
	return store.Notification{}, nil
}
func (m *mockStore) ListNotificationsByProfile(ctx context.Context, arg store.ListNotificationsByProfileParams) ([]store.Notification, error) {
	if m.listNotificationsByProfile != nil {
		return m.listNotificationsByProfile(ctx, arg)
	}
	return nil, nil
}
func (m *mockStore) CountUnreadNotifications(ctx context.Context, profileID uuid.UUID) (int64, error) {
	if m.countUnreadNotifications != nil {
		return m.countUnreadNotifications(ctx, profileID)
	}
	return 0, nil
}
func (m *mockStore) MarkNotificationRead(ctx context.Context, arg store.MarkNotificationReadParams) error {
	if m.markNotificationRead != nil {
		return m.markNotificationRead(ctx, arg)
	}
	return nil
}
func (m *mockStore) MarkAllNotificationsRead(ctx context.Context, profileID uuid.UUID) error {
	if m.markAllNotificationsRead != nil {
		return m.markAllNotificationsRead(ctx, profileID)
	}
	return nil
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
