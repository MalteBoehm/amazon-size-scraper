CREATE TABLE IF NOT EXISTS products (
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
CREATE INDEX idx_products_status ON products(status);

-- Index for timestamp queries
CREATE INDEX idx_products_created_at ON products(created_at);
CREATE INDEX idx_products_scraped_at ON products(scraped_at);

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_products_updated_at BEFORE UPDATE
    ON products FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();