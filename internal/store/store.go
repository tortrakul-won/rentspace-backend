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
	RenterDisplayName   string         `json:"renter_display_name"`
	RenterFullName      string         `json:"renter_full_name"`
	RenterPhone         sql.NullString `json:"renter_phone"`
	OwnerDisplayName    string         `json:"owner_display_name"`
	OwnerFullName       string         `json:"owner_full_name"`
}

const getAdminBookingDetail = `
SELECT
  b.id, b.space_id, b.renter_id, b.start_time, b.end_time, b.total_price, b.platform_fee,
  b.status, b.created_at, b.updated_at, b.cancel_reason, b.refund_status,
  b.process_expires_at, b.slip_url, b.ref_code,
  s.name AS space_name, s.location AS space_location, s.images AS space_images,
  renter_p.display_name AS renter_display_name,
  renter_u.full_name AS renter_full_name,
  renter_u.phone AS renter_phone,
  owner_p.display_name AS owner_display_name,
  owner_u.full_name AS owner_full_name
FROM bookings b
JOIN spaces s ON s.id = b.space_id
JOIN profiles renter_p ON renter_p.id = b.renter_id
JOIN users renter_u ON renter_u.id = renter_p.user_id
JOIN profiles owner_p ON owner_p.id = s.owner_id
JOIN users owner_u ON owner_u.id = owner_p.user_id
WHERE b.id = $1
`

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
		&r.RenterDisplayName,
		&r.RenterFullName,
		&r.RenterPhone,
		&r.OwnerDisplayName,
		&r.OwnerFullName,
	)
	return r, err
}
