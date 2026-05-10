UPDATE spaces SET min_minutes = min_minutes / 60;
ALTER TABLE spaces RENAME COLUMN min_minutes TO min_hours;
