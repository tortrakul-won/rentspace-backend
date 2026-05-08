CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE user_role AS ENUM ('owner', 'renter', 'both');
CREATE TYPE renter_type AS ENUM ('individual', 'company');
CREATE TYPE booking_status AS ENUM ('pending', 'confirmed', 'completed', 'cancelled');
CREATE TYPE space_category AS ENUM ('Studio', 'Outdoor', 'Loft', 'Garden', 'Office', 'Café');

CREATE TABLE users (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email        TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  full_name    TEXT NOT NULL,
  phone        TEXT,
  role         user_role NOT NULL DEFAULT 'renter',
  -- tax profile (required for document generation)
  tax_id       TEXT,
  is_juristic  BOOLEAN NOT NULL DEFAULT FALSE,
  is_vat_registered BOOLEAN NOT NULL DEFAULT FALSE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE spaces (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  description  TEXT NOT NULL,
  location     TEXT NOT NULL,
  category     space_category NOT NULL,
  images       TEXT[] NOT NULL DEFAULT '{}',
  -- prices in satang (THB smallest unit)
  hourly_rate  INTEGER NOT NULL CHECK (hourly_rate > 0),
  daily_rate   INTEGER NOT NULL CHECK (daily_rate > 0),
  min_hours    INTEGER NOT NULL DEFAULT 1 CHECK (min_hours >= 1),
  capacity     INTEGER NOT NULL CHECK (capacity > 0),
  amenities    TEXT[] NOT NULL DEFAULT '{}',
  is_active    BOOLEAN NOT NULL DEFAULT TRUE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE bookings (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  space_id       UUID NOT NULL REFERENCES spaces(id) ON DELETE RESTRICT,
  renter_id      UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  start_time     TIMESTAMPTZ NOT NULL,
  end_time       TIMESTAMPTZ NOT NULL,
  -- prices in satang
  total_price    INTEGER NOT NULL CHECK (total_price > 0),
  platform_fee   INTEGER NOT NULL CHECK (platform_fee >= 0),
  status         booking_status NOT NULL DEFAULT 'pending',
  -- renter tax info snapshot at booking time (immutable after creation)
  renter_type    renter_type NOT NULL DEFAULT 'individual',
  company_name   TEXT,
  tax_id         TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT valid_time_range CHECK (end_time > start_time)
);

-- prevent double-booking: no two confirmed/pending bookings can overlap on the same space
CREATE UNIQUE INDEX no_overlapping_bookings ON bookings (space_id, tstzrange(start_time, end_time))
  WHERE status IN ('pending', 'confirmed');

CREATE INDEX idx_spaces_owner ON spaces(owner_id);
CREATE INDEX idx_spaces_category ON spaces(category);
CREATE INDEX idx_bookings_space ON bookings(space_id);
CREATE INDEX idx_bookings_renter ON bookings(renter_id);
CREATE INDEX idx_bookings_status ON bookings(status);
