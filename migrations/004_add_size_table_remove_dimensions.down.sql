-- Re-add the individual dimension columns
ALTER TABLE product ADD COLUMN height_cm NUMERIC(10,2);
ALTER TABLE product ADD COLUMN length_cm NUMERIC(10,2);
ALTER TABLE product ADD COLUMN width_cm NUMERIC(10,2);
ALTER TABLE product ADD COLUMN weight_g NUMERIC(10,2);

-- Re-add constraints and indexes
ALTER TABLE product ADD CONSTRAINT chk_positive_dimensions CHECK (
    (height_cm IS NULL OR height_cm > 0) AND
    (length_cm IS NULL OR length_cm > 0) AND
    (width_cm IS NULL OR width_cm > 0) AND
    (weight_g IS NULL OR weight_g > 0)
);

CREATE INDEX idx_product_dimensions ON product (height_cm, length_cm, width_cm);
CREATE INDEX idx_product_dimensions_composite ON product (height_cm, length_cm) 
    WHERE height_cm IS NOT NULL AND length_cm IS NOT NULL;

-- Drop the size_table column
DROP INDEX IF EXISTS idx_product_size_table;
ALTER TABLE product DROP COLUMN IF EXISTS size_table;