DELETE FROM system_config WHERE key IN ('promptpay_number', 'promptpay_name');

ALTER TABLE users DROP COLUMN IF EXISTS is_admin;

-- Postgres cannot remove enum values without recreating the type.
-- Cancel any payment_pending bookings so the value is safe to ignore.
UPDATE bookings SET status = 'cancelled', updated_at = NOW() WHERE status = 'payment_pending';
