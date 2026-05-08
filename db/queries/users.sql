-- name: CreateUser :one
INSERT INTO users (email, password_hash, full_name, phone, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUserTaxProfile :one
UPDATE users SET
  tax_id            = $2,
  is_juristic       = $3,
  is_vat_registered = $4,
  updated_at        = NOW()
WHERE id = $1
RETURNING *;
