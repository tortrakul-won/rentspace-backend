ALTER TABLE bookings ADD COLUMN ref_code TEXT NOT NULL DEFAULT '';

UPDATE bookings SET ref_code = upper(substring(replace(id::text, '-', ''), 1, 8)) WHERE ref_code = '';

ALTER TABLE bookings ALTER COLUMN ref_code DROP DEFAULT;
ALTER TABLE bookings ADD CONSTRAINT bookings_ref_code_unique UNIQUE (ref_code);
