INSERT INTO system_config (key, value) VALUES
  ('vat_rate_pct',        '7'),
  ('platform_name_th',    'บริษัท เรนท์สเปซ จำกัด'),
  ('platform_name_en',    'RentSpace Co., Ltd.'),
  ('platform_tax_id',     '0000000000000'),
  ('platform_address',    '(fill before go-live)'),
  ('platform_branch_number', '00000')
ON CONFLICT (key) DO NOTHING;
