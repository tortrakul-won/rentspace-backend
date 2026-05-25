ALTER TYPE booking_status ADD VALUE IF NOT EXISTS 'awaiting_payment' AFTER 'payment_pending';
ALTER TYPE booking_status ADD VALUE IF NOT EXISTS 'payment_review' AFTER 'awaiting_payment';

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS refund_status TEXT CHECK (refund_status IN ('pending', 'issued')) DEFAULT NULL;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS process_expires_at TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS slip_url TEXT DEFAULT NULL;
