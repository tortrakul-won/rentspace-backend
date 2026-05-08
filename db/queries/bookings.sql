-- name: CreateBooking :one
INSERT INTO bookings (space_id, renter_id, start_time, end_time, total_price, platform_fee)
VALUES ($1, $2, $3, $4, $5, $6)
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

-- name: UpdateBookingStatus :one
UPDATE bookings SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CheckOverlappingBookings :one
SELECT COUNT(*) FROM bookings
WHERE space_id   = $1
  AND status     IN ('pending', 'confirmed')
  AND start_time < $3
  AND end_time   > $2;
