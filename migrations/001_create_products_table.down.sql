DROP TRIGGER IF EXISTS update_product_updated_at ON product;
DROP INDEX IF EXISTS idx_product_scraped_at;
DROP INDEX IF EXISTS idx_product_created_at;
DROP INDEX IF EXISTS idx_product_status;
DROP TABLE IF EXISTS product;