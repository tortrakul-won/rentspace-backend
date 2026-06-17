ALTER TABLE bookings
  DROP COLUMN IF EXISTS vat_rate_pct,
  DROP COLUMN IF EXISTS renter_accepted_at,
  DROP COLUMN IF EXISTS owner_accepted_at;
