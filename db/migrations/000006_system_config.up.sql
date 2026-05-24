CREATE TABLE system_config (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO system_config (key, value) VALUES
  ('max_booking_minutes',            '10080'),
  ('default_lead_time_hours',        '4'),
  ('max_pending_bookings_per_renter','5'),
  ('platform_fee_pct',               '10');
