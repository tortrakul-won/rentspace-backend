ALTER TABLE bookings
  ADD COLUMN IF NOT EXISTS vat_rate_pct       INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS renter_accepted_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS owner_accepted_at  TIMESTAMPTZ;

UPDATE bookings SET renter_accepted_at = created_at WHERE renter_accepted_at IS NULL;
