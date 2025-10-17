-- Create scraper_jobs table for tracking scraping jobs
CREATE TABLE IF NOT EXISTS scraper_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    search_query TEXT NOT NULL,
    category VARCHAR(50),
    max_pages INT DEFAULT 10,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    pages_scraped INT DEFAULT 0,
    products_found INT DEFAULT 0,
    products_complete INT DEFAULT 0,
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX idx_scraper_jobs_status ON scraper_jobs(status);
CREATE INDEX idx_scraper_jobs_created_at ON scraper_jobs(created_at DESC);

-- Create job_products table to track which products were found by which job
CREATE TABLE IF NOT EXISTS job_products (
    job_id UUID REFERENCES scraper_jobs(id) ON DELETE CASCADE,
    asin VARCHAR(20) REFERENCES products(asin) ON DELETE CASCADE,
    page_number INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (job_id, asin)
);

-- Create index for faster lookups
CREATE INDEX idx_job_products_asin ON job_products(asin);

-- Add comment to tables
COMMENT ON TABLE scraper_jobs IS 'Tracks Amazon category/search scraping jobs';
COMMENT ON TABLE job_products IS 'Links products to the jobs that discovered them';