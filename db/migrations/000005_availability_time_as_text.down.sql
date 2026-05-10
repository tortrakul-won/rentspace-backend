ALTER TABLE space_availability DROP CONSTRAINT IF EXISTS valid_hours;
ALTER TABLE space_availability
  ALTER COLUMN open_time  TYPE TIME USING open_time::TIME,
  ALTER COLUMN close_time TYPE TIME USING close_time::TIME;
ALTER TABLE space_availability ADD CONSTRAINT valid_hours CHECK (close_time > open_time);
