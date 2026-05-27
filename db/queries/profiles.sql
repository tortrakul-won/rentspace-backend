-- name: CreateProfile :one
INSERT INTO profiles (user_id, role, profile_name, legal_name_th, legal_name_en, phone, address_line1, subdistrict, district, province, postal_code, branch_number, tax_id, is_juristic, is_vat_registered, line_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: GetProfilesByUserID :many
SELECT * FROM profiles WHERE user_id = $1 AND is_active = TRUE ORDER BY created_at ASC;

-- name: GetProfileByID :one
SELECT * FROM profiles WHERE id = $1;

-- name: GetProfileByUserAndRole :one
SELECT * FROM profiles WHERE user_id = $1 AND role = $2;

-- name: UpdateProfile :one
UPDATE profiles SET
  profile_name   = $2,
  legal_name_th  = $3,
  legal_name_en  = $4,
  phone          = $5,
  address_line1  = $6,
  subdistrict    = $7,
  district       = $8,
  province       = $9,
  postal_code    = $10,
  branch_number  = $11,
  tax_id         = $12,
  is_juristic    = $13,
  is_vat_registered = $14,
  line_id        = $15,
  updated_at     = NOW()
WHERE id = $1
RETURNING *;
