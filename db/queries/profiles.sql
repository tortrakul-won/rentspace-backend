-- name: CreateProfile :one
INSERT INTO profiles (user_id, role, display_name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetProfilesByUserID :many
SELECT * FROM profiles WHERE user_id = $1 AND is_active = TRUE ORDER BY created_at ASC;

-- name: GetProfileByID :one
SELECT * FROM profiles WHERE id = $1;

-- name: GetProfileByUserAndRole :one
SELECT * FROM profiles WHERE user_id = $1 AND role = $2;

-- name: UpdateProfile :one
UPDATE profiles SET display_name = $2, line_id = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;
