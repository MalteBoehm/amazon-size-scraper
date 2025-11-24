-- Add material composition fields to product table
ALTER TABLE product
ADD COLUMN material_composition JSONB,
ADD COLUMN material_full_text TEXT;

-- Create indexes for material fields
CREATE INDEX idx_product_material_composition ON product USING gin (material_composition);
CREATE INDEX idx_product_material_full_text ON product USING gin (to_tsvector('german', COALESCE(material_full_text, '')));

-- Add comments
COMMENT ON COLUMN product.material_composition IS 'Structured material data extracted from product page';
COMMENT ON COLUMN product.material_full_text IS 'Full material text for learning and fallback purposes';