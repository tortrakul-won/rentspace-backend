ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_ref_code_unique;
ALTER TABLE bookings DROP COLUMN IF EXISTS ref_code;
