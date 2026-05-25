DROP INDEX IF EXISTS idx_notifications_booking_id;
ALTER TABLE notifications DROP COLUMN IF EXISTS booking_id;
