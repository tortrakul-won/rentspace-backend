-- name: GetSpaceAvailability :many
SELECT * FROM space_availability
WHERE space_id = $1
ORDER BY day_of_week;

-- name: UpsertSpaceAvailability :one
INSERT INTO space_availability (space_id, day_of_week, open_time, close_time)
VALUES ($1, $2, $3, $4)
ON CONFLICT (space_id, day_of_week) DO UPDATE SET
  open_time  = EXCLUDED.open_time,
  close_time = EXCLUDED.close_time
RETURNING *;

-- name: DeleteSpaceAvailability :exec
DELETE FROM space_availability
WHERE space_id = $1;
