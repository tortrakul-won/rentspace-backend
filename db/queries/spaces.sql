-- name: ListSpaces :many
SELECT * FROM spaces
WHERE is_active = TRUE
ORDER BY created_at DESC;

-- name: ListSpacesPaginated :many
SELECT * FROM spaces
WHERE is_active = TRUE
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountSpaces :one
SELECT COUNT(*) FROM spaces WHERE is_active = TRUE;

-- name: ListSpacesByCategory :many
SELECT * FROM spaces
WHERE is_active = TRUE AND category = @category
ORDER BY created_at DESC;

-- name: GetSpaceByID :one
SELECT * FROM spaces
WHERE id = $1;

-- name: ListSpacesByOwner :many
SELECT * FROM spaces
WHERE owner_id = $1
ORDER BY created_at DESC;

-- name: ListSpacesByOwnerPaginated :many
SELECT * FROM spaces
WHERE owner_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountSpacesByOwner :one
SELECT COUNT(*) FROM spaces WHERE owner_id = $1;

-- name: CreateSpace :one
INSERT INTO spaces (owner_id, name, description, location, category, images, hourly_rate, daily_rate, min_hours, capacity, amenities, weekend_surcharge_pct)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateSpace :one
UPDATE spaces SET
  name                  = $2,
  description           = $3,
  location              = $4,
  category              = $5,
  images                = $6,
  hourly_rate           = $7,
  daily_rate            = $8,
  min_hours             = $9,
  capacity              = $10,
  amenities             = $11,
  weekend_surcharge_pct = $12,
  updated_at            = NOW()
WHERE id = $1 AND owner_id = $13
RETURNING *;

-- name: SetSpaceActive :one
UPDATE spaces SET is_active = $2, updated_at = NOW()
WHERE id = $1 AND owner_id = $3
RETURNING *;
