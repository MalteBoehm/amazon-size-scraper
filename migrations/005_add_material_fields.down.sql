-- Remove indexes first
DROP INDEX IF EXISTS idx_products_material_composition;
DROP INDEX IF EXISTS idx_products_material_full_text;

-- Remove material columns from product table
ALTER TABLE products
DROP COLUMN IF EXISTS material_composition,
DROP COLUMN IF EXISTS material_full_text;