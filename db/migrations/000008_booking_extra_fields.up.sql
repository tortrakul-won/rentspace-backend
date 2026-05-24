ALTER TABLE bookings
  ADD COLUMN headcount INTEGER,
  ADD COLUMN notes     TEXT,
  ADD COLUMN expires_at TIMESTAMPTZ;
