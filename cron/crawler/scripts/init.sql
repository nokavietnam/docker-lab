-- Schema for Real Estate Crawler Lab (PostgreSQL)

CREATE TABLE IF NOT EXISTS properties (
    id BIGINT PRIMARY KEY,
    title TEXT NOT NULL,
    summary TEXT,
    price BIGINT DEFAULT 0,
    price_display VARCHAR(64),
    category_id INT,
    category_name VARCHAR(128),
    city_id INT,
    district_id INT,
    location TEXT,
    url TEXT,
    phone_display VARCHAR(64),
    publish_at TIMESTAMPTZ,
    attributes JSONB,
    covers JSONB,
    locations_display JSONB,
    raw_data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexing Strategy for High Query Performance in System Design Labs
CREATE INDEX IF NOT EXISTS idx_properties_price ON properties (price);
CREATE INDEX IF NOT EXISTS idx_properties_publish_at ON properties (publish_at DESC);
CREATE INDEX IF NOT EXISTS idx_properties_location_ids ON properties (city_id, district_id);
CREATE INDEX IF NOT EXISTS idx_properties_raw_data_gin ON properties USING gin (raw_data);

-- Checkpoint Table for Resumable & Idempotent Crawling
CREATE TABLE IF NOT EXISTS crawler_checkpoints (
    category_key VARCHAR(128) PRIMARY KEY,
    last_page INT NOT NULL DEFAULT 1,
    total_pages INT NOT NULL DEFAULT 0,
    total_items INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'running',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
