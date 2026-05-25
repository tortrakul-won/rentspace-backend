UPDATE bookings SET status = 'payment_pending' WHERE status IN ('awaiting_payment', 'payment_review');

ALTER TABLE spaces DROP COLUMN IF EXISTS payment_deadline_minutes;
ALTER TABLE profiles DROP COLUMN IF EXISTS line_id;

DELETE FROM system_config WHERE key IN ('payment_deadline_minutes', 'payment_process_deadline_minutes', 'payment_warning_minutes');
