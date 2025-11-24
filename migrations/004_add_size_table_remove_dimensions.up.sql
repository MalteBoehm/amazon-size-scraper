-- Add size_table column for storing complete size variations
ALTER TABLE product ADD COLUMN size_table JSONB;

-- Drop the individual dimension columns and related constraints/indexes
ALTER TABLE product DROP CONSTRAINT IF EXISTS chk_positive_dimensions;
DROP INDEX IF EXISTS idx_product_dimensions;
DROP INDEX IF EXISTS idx_product_dimensions_composite;

ALTER TABLE product DROP COLUMN IF EXISTS height_cm;
ALTER TABLE product DROP COLUMN IF EXISTS length_cm;
ALTER TABLE product DROP COLUMN IF EXISTS width_cm;
ALTER TABLE product DROP COLUMN IF EXISTS weight_g;

-- Add an index on the new size_table column for JSON queries
CREATE INDEX idx_product_size_table ON product USING GIN (size_table);