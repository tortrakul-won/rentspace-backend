ALTER TABLE space_availability DROP CONSTRAINT IF EXISTS valid_hours;
ALTER TABLE space_availability
  ALTER COLUMN open_time  TYPE TEXT USING to_char(open_time, 'HH24:MI'),
  ALTER COLUMN close_time TYPE TEXT USING to_char(close_time, 'HH24:MI');
ALTER TABLE space_availability ADD CONSTRAINT valid_hours CHECK (close_time > open_time);
