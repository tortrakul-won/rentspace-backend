ALTER TABLE spaces
  DROP COLUMN IF EXISTS min_notice_hours,
  DROP COLUMN IF EXISTS max_booking_minutes,
  DROP COLUMN IF EXISTS turnaround_minutes,
  DROP COLUMN IF EXISTS deposit_pct;
