-- Remove individual dimension columns as we now store complete size tables
ALTER TABLE products 
DROP COLUMN IF EXISTS width_cm,
DROP COLUMN IF EXISTS length_cm,
DROP COLUMN IF EXISTS height_cm;