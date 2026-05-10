ALTER TABLE spaces RENAME COLUMN min_hours TO min_minutes;
UPDATE spaces SET min_minutes = min_minutes * 60;
