ALTER TABLE profiles DROP COLUMN IF EXISTS legal_name_en;

ALTER TABLE profiles RENAME COLUMN legal_name_th TO legal_name;
