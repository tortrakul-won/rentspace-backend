package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Store extends Querier with transactional execution and batch config reads.
type Store interface {
	Querier
	ExecTx(ctx context.Context, fn func(Querier) error) error
	// GetSystemConfigMultiple fetches multiple system_config keys in one round-trip.
	// Missing keys are absent from the returned map (no error).
	GetSystemConfigMultiple(ctx context.Context, keys []string) (map[string]string, error)
	// GetAdminBookingDetail returns a fully-enriched booking row for the admin review page.
	GetAdminBookingDetail(ctx context.Context, id uuid.UUID) (AdminBookingDetailRow, error)
	// ListAdminProfileIDs returns all profile IDs belonging to admin users.
	ListAdminProfileIDs(ctx context.Context) ([]uuid.UUID, error)
	// GetOwnerBookingDetail returns a booking enriched with renter info, scoped to the owner's spaces.
	GetOwnerBookingDetail(ctx context.Context, id uuid.UUID, ownerProfileID uuid.UUID) (OwnerBookingDetailRow, error)
	// GetRenterBookingDetail returns a booking enriched with owner/space info, scoped to the renter.
	GetRenterBookingDetail(ctx context.Context, id uuid.UUID, renterProfileID uuid.UUID) (RenterBookingDetailRow, error)
}

// SQLStore is the production implementation backed by *sql.DB.
type SQLStore struct {
	*Queries
	db *sql.DB
}

func NewStore(db *sql.DB) *SQLStore {
	return &SQLStore{Queries: New(db), db: db}
}

// ExecTx runs fn inside a transaction. If fn returns an error the transaction
// is rolled back; otherwise it is committed.
func (s *SQLStore) ExecTx(ctx context.Context, fn func(Querier) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(s.WithTx(tx)); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %w, rollback err: %v", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

// GetSystemConfigMultiple fetches multiple system_config keys in one query.
// Missing keys are absent from the returned map (no error).
func (s *SQLStore) GetSystemConfigMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, k := range keys {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = k
	}
	query := "SELECT key, value FROM system_config WHERE key IN (" + strings.Join(placeholders, ", ") + ")"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string, len(keys))
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

// AdminBookingDetailRow is a fully-enriched booking for the admin review page.
type AdminBookingDetailRow struct {
	ID                  uuid.UUID      `json:"id"`
	SpaceID             uuid.UUID      `json:"space_id"`
	RenterID            uuid.UUID      `json:"renter_id"`
	StartTime           time.Time      `json:"start_time"`
	EndTime             time.Time      `json:"end_time"`
	TotalPrice          int32          `json:"total_price"`
	PlatformFee         int32          `json:"platform_fee"`
	Status              BookingStatus  `json:"status"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	CancelReason        sql.NullString `json:"cancel_reason"`
	RefundStatus        sql.NullString `json:"refund_status"`
	ProcessExpiresAt    sql.NullTime   `json:"process_expires_at"`
	SlipUrl             sql.NullString `json:"slip_url"`
	RefCode             string         `json:"ref_code"`
	SpaceName           string         `json:"space_name"`
	SpaceLocation       string         `json:"space_location"`
	SpaceImages         []string       `json:"space_images"`
	RenterProfileName   string `json:"renter_profile_name"`
	RenterPhone         string `json:"renter_phone"`
	RenterIsJuristic    bool   `json:"renter_is_juristic"`
	OwnerProfileName    string `json:"owner_profile_name"`
}

const getAdminBookingDetail = `
SELECT
  b.id, b.space_id, b.renter_id, b.start_time, b.end_time, b.total_price, b.platform_fee,
  b.status, b.created_at, b.updated_at, b.cancel_reason, b.refund_status,
  b.process_expires_at, b.slip_url, b.ref_code,
  s.name AS space_name, s.location AS space_location, s.images AS space_images,
  renter_p.profile_name AS renter_profile_name,
  renter_p.phone AS renter_phone,
  renter_p.is_juristic AS renter_is_juristic,
  owner_p.profile_name AS owner_profile_name
FROM bookings b
JOIN spaces s ON s.id = b.space_id
JOIN profiles renter_p ON renter_p.id = b.renter_id
JOIN profiles owner_p ON owner_p.id = s.owner_id
WHERE b.id = $1
`

func (s *SQLStore) ListAdminProfileIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id FROM profiles p
		JOIN users u ON u.id = p.user_id
		WHERE u.is_admin = TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// OwnerBookingDetailRow is an enriched booking row for the owner detail page.
type OwnerBookingDetailRow struct {
	ID                uuid.UUID      `json:"id"`
	SpaceID           uuid.UUID      `json:"space_id"`
	RenterID          uuid.UUID      `json:"renter_id"`
	StartTime         time.Time      `json:"start_time"`
	EndTime           time.Time      `json:"end_time"`
	TotalPrice        int32          `json:"total_price"`
	PlatformFee       int32          `json:"platform_fee"`
	Status            BookingStatus  `json:"status"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	CancelReason      sql.NullString `json:"cancel_reason"`
	RefCode           string         `json:"ref_code"`
	SpaceName         string         `json:"space_name"`
	SpaceLocation     string         `json:"space_location"`
	SpaceImages       []string       `json:"space_images"`
	RenterProfileName string `json:"renter_profile_name"`
	RenterPhone       string `json:"renter_phone"`
	RenterIsJuristic  bool   `json:"renter_is_juristic"`
}

const getOwnerBookingDetail = `
SELECT
  b.id, b.space_id, b.renter_id, b.start_time, b.end_time, b.total_price, b.platform_fee,
  b.status, b.created_at, b.updated_at, b.cancel_reason, b.ref_code,
  s.name AS space_name, s.location AS space_location, s.images AS space_images,
  rp.profile_name AS renter_profile_name,
  rp.phone AS renter_phone,
  rp.is_juristic AS renter_is_juristic
FROM bookings b
JOIN spaces s ON s.id = b.space_id
JOIN profiles rp ON rp.id = b.renter_id
WHERE b.id = $1 AND s.owner_id = $2
`

func (s *SQLStore) GetOwnerBookingDetail(ctx context.Context, id uuid.UUID, ownerProfileID uuid.UUID) (OwnerBookingDetailRow, error) {
	var r OwnerBookingDetailRow
	err := s.db.QueryRowContext(ctx, getOwnerBookingDetail, id, ownerProfileID).Scan(
		&r.ID,
		&r.SpaceID,
		&r.RenterID,
		&r.StartTime,
		&r.EndTime,
		&r.TotalPrice,
		&r.PlatformFee,
		&r.Status,
		&r.CreatedAt,
		&r.UpdatedAt,
		&r.CancelReason,
		&r.RefCode,
		&r.SpaceName,
		&r.SpaceLocation,
		pq.Array(&r.SpaceImages),
		&r.RenterProfileName,
		&r.RenterPhone,
		&r.RenterIsJuristic,
	)
	return r, err
}

func (s *SQLStore) GetAdminBookingDetail(ctx context.Context, id uuid.UUID) (AdminBookingDetailRow, error) {
	var r AdminBookingDetailRow
	err := s.db.QueryRowContext(ctx, getAdminBookingDetail, id).Scan(
		&r.ID,
		&r.SpaceID,
		&r.RenterID,
		&r.StartTime,
		&r.EndTime,
		&r.TotalPrice,
		&r.PlatformFee,
		&r.Status,
		&r.CreatedAt,
		&r.UpdatedAt,
		&r.CancelReason,
		&r.RefundStatus,
		&r.ProcessExpiresAt,
		&r.SlipUrl,
		&r.RefCode,
		&r.SpaceName,
		&r.SpaceLocation,
		pq.Array(&r.SpaceImages),
		&r.RenterProfileName,
		&r.RenterPhone,
		&r.RenterIsJuristic,
		&r.OwnerProfileName,
	)
	return r, err
}

// RenterBookingDetailRow is an enriched booking for the renter detail page.
type RenterBookingDetailRow struct {
	ID               uuid.UUID      `json:"id"`
	SpaceID          uuid.UUID      `json:"space_id"`
	RenterID         uuid.UUID      `json:"renter_id"`
	StartTime        time.Time      `json:"start_time"`
	EndTime          time.Time      `json:"end_time"`
	TotalPrice       int32          `json:"total_price"`
	PlatformFee      int32          `json:"platform_fee"`
	Status           BookingStatus  `json:"status"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	CancelReason     sql.NullString `json:"cancel_reason"`
	RefundStatus     sql.NullString `json:"refund_status"`
	ProcessExpiresAt sql.NullTime   `json:"process_expires_at"`
	SlipUrl          sql.NullString `json:"slip_url"`
	RefCode          string         `json:"ref_code"`
	SpaceName        string         `json:"space_name"`
	SpaceLocation    string         `json:"space_location"`
	SpaceImages      []string       `json:"space_images"`
	OwnerProfileName string         `json:"owner_profile_name"`
	OwnerPhone       string         `json:"owner_phone"`
	OwnerLineID      string         `json:"owner_line_id"`
}

const getRenterBookingDetail = `
SELECT
  b.id, b.space_id, b.renter_id, b.start_time, b.end_time, b.total_price, b.platform_fee,
  b.status, b.created_at, b.updated_at, b.cancel_reason, b.refund_status,
  b.process_expires_at, b.slip_url, b.ref_code,
  s.name AS space_name, s.location AS space_location, s.images AS space_images,
  op.profile_name AS owner_profile_name,
  op.phone AS owner_phone,
  COALESCE(op.line_id, '') AS owner_line_id
FROM bookings b
JOIN spaces s ON s.id = b.space_id
JOIN profiles op ON op.id = s.owner_id
WHERE b.id = $1 AND b.renter_id = $2
`

func (s *SQLStore) GetRenterBookingDetail(ctx context.Context, id uuid.UUID, renterProfileID uuid.UUID) (RenterBookingDetailRow, error) {
	var r RenterBookingDetailRow
	err := s.db.QueryRowContext(ctx, getRenterBookingDetail, id, renterProfileID).Scan(
		&r.ID,
		&r.SpaceID,
		&r.RenterID,
		&r.StartTime,
		&r.EndTime,
		&r.TotalPrice,
		&r.PlatformFee,
		&r.Status,
		&r.CreatedAt,
		&r.UpdatedAt,
		&r.CancelReason,
		&r.RefundStatus,
		&r.ProcessExpiresAt,
		&r.SlipUrl,
		&r.RefCode,
		&r.SpaceName,
		&r.SpaceLocation,
		pq.Array(&r.SpaceImages),
		&r.OwnerProfileName,
		&r.OwnerPhone,
		&r.OwnerLineID,
	)
	return r, err
}
