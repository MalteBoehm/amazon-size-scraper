CREATE TABLE IF NOT EXISTS product (
    asin VARCHAR(20) PRIMARY KEY,
    title TEXT NOT NULL,
    brand VARCHAR(255),
    category VARCHAR(255),
    url TEXT NOT NULL,
    
    -- Size data stored as JSON
    size_table JSONB,
    
    -- Extracted dimensions
    width_cm DECIMAL(10,2),
    length_cm DECIMAL(10,2),
    height_cm DECIMAL(10,2),
    
    -- Status tracking
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    
    -- Timestamps
    scraped_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Index for status queries
CREATE INDEX idx_product_status ON product(status);

-- Index for timestamp queries
CREATE INDEX idx_product_created_at ON product(created_at);
CREATE INDEX idx_product_scraped_at ON product(scraped_at);

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_product_updated_at BEFORE UPDATE
    ON product FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();