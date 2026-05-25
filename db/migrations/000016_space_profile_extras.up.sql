-- Data migration: existing payment_pending bookings move to awaiting_payment.
-- Must run after 000015 so the new enum values are committed.
UPDATE bookings SET status = 'awaiting_payment' WHERE status = 'payment_pending';

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS payment_deadline_minutes INT DEFAULT NULL;
ALTER TABLE profiles ADD COLUMN IF NOT EXISTS line_id VARCHAR(64) DEFAULT NULL;

INSERT INTO system_config (key, value) VALUES
  ('payment_deadline_minutes',         '60'),
  ('payment_process_deadline_minutes', '1440'),
  ('payment_warning_minutes',          '15')
ON CONFLICT (key) DO NOTHING;
