-- name: CreateNotification :one
INSERT INTO notifications (profile_id, type, payload)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListNotificationsByProfile :many
SELECT * FROM notifications
WHERE profile_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications
WHERE profile_id = $1 AND read_at IS NULL;

-- name: MarkNotificationRead :exec
UPDATE notifications SET read_at = NOW()
WHERE id = $1 AND profile_id = $2;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET read_at = NOW()
WHERE profile_id = $1 AND read_at IS NULL;
