-- Add material composition fields to product table
ALTER TABLE products
ADD COLUMN material_composition JSONB,
ADD COLUMN material_full_text TEXT;

-- Create indexes for material fields
CREATE INDEX idx_products_material_composition ON products USING gin (material_composition);
CREATE INDEX idx_products_material_full_text ON products USING gin (to_tsvector('german', COALESCE(material_full_text, '')));

-- Add comments
COMMENT ON COLUMN products.material_composition IS 'Structured material data extracted from product page';
COMMENT ON COLUMN products.material_full_text IS 'Full material text for learning and fallback purposes';