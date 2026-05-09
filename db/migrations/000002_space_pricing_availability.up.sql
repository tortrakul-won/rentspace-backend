ALTER TABLE spaces
  ADD COLUMN weekend_surcharge_pct INTEGER NOT NULL DEFAULT 0
    CHECK (weekend_surcharge_pct >= 0 AND weekend_surcharge_pct <= 100);

CREATE TABLE space_availability (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  space_id    UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  day_of_week SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
  open_time   TIME NOT NULL,
  close_time  TIME NOT NULL,
  CONSTRAINT valid_hours CHECK (close_time > open_time),
  CONSTRAINT unique_space_day UNIQUE (space_id, day_of_week)
);

CREATE INDEX idx_availability_space ON space_availability(space_id);
