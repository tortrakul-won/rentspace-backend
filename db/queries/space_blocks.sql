-- name: CreateSpaceBlock :one
INSERT INTO space_blocks (space_id, start_time, end_time, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListSpaceBlocksBySpace :many
SELECT * FROM space_blocks
WHERE space_id = $1
ORDER BY start_time ASC;

-- name: GetSpaceBlocksInRange :many
SELECT * FROM space_blocks
WHERE space_id   = $1
  AND start_time < $3
  AND end_time   > $2
ORDER BY start_time ASC;

-- name: CheckOverlappingSpaceBlocks :one
SELECT COUNT(*) FROM space_blocks
WHERE space_id   = $1
  AND start_time < $3
  AND end_time   > $2;

-- name: DeleteSpaceBlock :exec
DELETE FROM space_blocks WHERE id = $1 AND space_id = $2;
