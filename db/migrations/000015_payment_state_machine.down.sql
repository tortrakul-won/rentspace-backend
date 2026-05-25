-- NOTE: Removing enum values is not possible in PostgreSQL.
-- The 'awaiting_payment' and 'payment_review' values remain in booking_status type after rollback.

ALTER TABLE bookings DROP COLUMN IF EXISTS refund_status;
ALTER TABLE bookings DROP COLUMN IF EXISTS process_expires_at;
ALTER TABLE bookings DROP COLUMN IF EXISTS slip_url;
