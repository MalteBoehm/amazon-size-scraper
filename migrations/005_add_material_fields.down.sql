DROP INDEX IF EXISTS idx_product_material_composition;
DROP INDEX IF EXISTS idx_product_material_full_text;

ALTER TABLE product
DROP COLUMN IF EXISTS material_composition,
DROP COLUMN IF EXISTS material_full_text;