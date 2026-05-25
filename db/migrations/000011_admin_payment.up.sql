ALTER TYPE booking_status ADD VALUE 'payment_pending' AFTER 'pending';

ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;

INSERT INTO system_config (key, value) VALUES
  ('promptpay_number', '0812345678'),
  ('promptpay_name',   'RentSpace Co., Ltd.')
ON CONFLICT (key) DO NOTHING;
