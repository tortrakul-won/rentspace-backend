-- name: CreateUser :one
INSERT INTO users (email, password_hash, full_name, phone)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: SetUserAdmin :one
UPDATE users SET is_admin = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUser :one
UPDATE users SET full_name = $2, phone = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;
