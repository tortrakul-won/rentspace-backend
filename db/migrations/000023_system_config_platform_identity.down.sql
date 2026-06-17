DELETE FROM system_config WHERE key IN (
  'vat_rate_pct',
  'platform_name_th',
  'platform_name_en',
  'platform_tax_id',
  'platform_address',
  'platform_branch_number'
);
