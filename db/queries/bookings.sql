-- name: CreateBooking :one
INSERT INTO bookings (space_id, renter_id, start_time, end_time, total_price, platform_fee, headcount, notes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetBookingByID :one
SELECT * FROM bookings WHERE id = $1;

-- name: ListBookingsByRenter :many
SELECT * FROM bookings
WHERE renter_id = $1
ORDER BY start_time DESC;

-- name: ListBookingsBySpace :many
SELECT * FROM bookings
WHERE space_id = $1
ORDER BY start_time DESC;

-- name: ListBookingsBySpacePaginated :many
SELECT * FROM bookings
WHERE space_id = $1
ORDER BY start_time DESC
LIMIT $2 OFFSET $3;

-- name: CountBookingsBySpace :one
SELECT COUNT(*) FROM bookings WHERE space_id = $1;

-- name: UpdateBookingStatus :one
UPDATE bookings SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListBookingsByOwner :many
SELECT b.* FROM bookings b
JOIN spaces s ON s.id = b.space_id
WHERE s.owner_id = $1
ORDER BY b.start_time DESC;

-- name: CheckOverlappingBookings :one
SELECT COUNT(*) FROM bookings
WHERE space_id   = $1
  AND status     IN ('pending', 'confirmed')
  AND start_time < $3
  AND end_time   > $2;

-- name: CountActiveBookingsByRenterForSpace :one
SELECT COUNT(*) FROM bookings
WHERE renter_id = $1
  AND space_id  = $2
  AND status    IN ('pending', 'confirmed');

-- name: CountPendingBookingsByRenter :one
SELECT COUNT(*) FROM bookings
WHERE renter_id = $1
  AND status    = 'pending';

-- name: BulkExpirePendingBookings :many
UPDATE bookings
SET status = 'cancelled', updated_at = NOW()
WHERE status     = 'pending'
  AND expires_at IS NOT NULL
  AND expires_at <= NOW()
RETURNING *;

-- name: BulkCompleteConfirmedBookings :many
UPDATE bookings
SET status = 'completed', updated_at = NOW()
WHERE status   = 'confirmed'
  AND end_time <= NOW()
RETURNING *;

-- name: ListActiveBookingsInRange :many
SELECT * FROM bookings
WHERE space_id   = $1
  AND status     IN ('pending', 'confirmed')
  AND start_time < $3
  AND end_time   > $2
ORDER BY start_time ASC;

-- name: CancelOverlappingPendingBookings :many
UPDATE bookings
SET status = 'cancelled', updated_at = NOW()
WHERE renter_id  = $1
  AND id        != $2
  AND status     = 'pending'
  AND start_time < $4
  AND end_time   > $3
RETURNING *;
