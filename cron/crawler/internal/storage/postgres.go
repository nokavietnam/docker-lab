package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"muaban-crawler/internal/model"
)

type Storage struct {
	pool *pgxpool.Pool
}

func NewStorage(ctx context.Context, dbURL string) (*Storage, error) {
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse db config: %w", err)
	}

	// Staff Engineer resource tuning for low-overhead local lab
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping to postgres failed: %w", err)
	}

	return &Storage{pool: pool}, nil
}

func (s *Storage) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// InitSchema ensures required tables and indexes exist
func (s *Storage) InitSchema(ctx context.Context) error {
	query := `
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

	CREATE INDEX IF NOT EXISTS idx_properties_price ON properties (price);
	CREATE INDEX IF NOT EXISTS idx_properties_publish_at ON properties (publish_at DESC);
	CREATE INDEX IF NOT EXISTS idx_properties_location_ids ON properties (city_id, district_id);
	CREATE INDEX IF NOT EXISTS idx_properties_raw_data_gin ON properties USING gin (raw_data);

	CREATE TABLE IF NOT EXISTS crawler_checkpoints (
		category_key VARCHAR(128) PRIMARY KEY,
		last_page INT NOT NULL DEFAULT 1,
		total_pages INT NOT NULL DEFAULT 0,
		total_items INT NOT NULL DEFAULT 0,
		status VARCHAR(32) NOT NULL DEFAULT 'running',
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	_, err := s.pool.Exec(ctx, query)
	return err
}

// SavePropertiesBatch performs an idempotent bulk upsert using pgx.Batch
func (s *Storage) SavePropertiesBatch(ctx context.Context, properties []model.Property) error {
	if len(properties) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	upsertQuery := `
	INSERT INTO properties (
		id, title, summary, price, price_display, category_id, category_name,
		city_id, district_id, location, url, phone_display, publish_at,
		attributes, covers, locations_display, raw_data, updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7,
		$8, $9, $10, $11, $12, $13,
		$14, $15, $16, $17, NOW()
	)
	ON CONFLICT (id) DO UPDATE SET
		title = EXCLUDED.title,
		summary = EXCLUDED.summary,
		price = EXCLUDED.price,
		price_display = EXCLUDED.price_display,
		location = EXCLUDED.location,
		phone_display = EXCLUDED.phone_display,
		publish_at = EXCLUDED.publish_at,
		attributes = EXCLUDED.attributes,
		covers = EXCLUDED.covers,
		locations_display = EXCLUDED.locations_display,
		raw_data = EXCLUDED.raw_data,
		updated_at = NOW();
	`

	for _, p := range properties {
		batch.Queue(upsertQuery,
			p.ID, p.Title, p.Summary, p.Price, p.PriceDisplay, p.CategoryID, p.CategoryName,
			p.CityID, p.DistrictID, p.Location, p.URL, p.PhoneDisplay, p.PublishAt,
			p.Attributes, p.Covers, p.LocationsDisplay, p.RawData,
		)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(properties); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch execution failed at item %d: %w", i, err)
		}
	}

	return nil
}

// GetCheckpoint retrieves the last crawl state
func (s *Storage) GetCheckpoint(ctx context.Context, categoryKey string) (*model.Checkpoint, error) {
	query := `SELECT category_key, last_page, total_pages, total_items, status, updated_at FROM crawler_checkpoints WHERE category_key = $1`
	row := s.pool.QueryRow(ctx, query, categoryKey)

	var cp model.Checkpoint
	err := row.Scan(&cp.CategoryKey, &cp.LastPage, &cp.TotalPages, &cp.TotalItems, &cp.Status, &cp.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &cp, nil
}

// SaveCheckpoint saves or updates progress for resumable operations
func (s *Storage) SaveCheckpoint(ctx context.Context, cp *model.Checkpoint) error {
	query := `
	INSERT INTO crawler_checkpoints (category_key, last_page, total_pages, total_items, status, updated_at)
	VALUES ($1, $2, $3, $4, $5, NOW())
	ON CONFLICT (category_key) DO UPDATE SET
		last_page = EXCLUDED.last_page,
		total_pages = EXCLUDED.total_pages,
		total_items = EXCLUDED.total_items,
		status = EXCLUDED.status,
		updated_at = NOW();
	`
	_, err := s.pool.Exec(ctx, query, cp.CategoryKey, cp.LastPage, cp.TotalPages, cp.TotalItems, cp.Status)
	return err
}

// CountProperties returns total records stored in DB
func (s *Storage) CountProperties(ctx context.Context) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM properties").Scan(&count)
	return count, err
}

// CheckExistingIDs checks a slice of property IDs against the database in a single indexed query
func (s *Storage) CheckExistingIDs(ctx context.Context, ids []int64) (map[int64]bool, error) {
	if len(ids) == 0 {
		return map[int64]bool{}, nil
	}

	query := `SELECT id FROM properties WHERE id = ANY($1)`
	rows, err := s.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing ids: %w", err)
	}
	defer rows.Close()

	existing := make(map[int64]bool, len(ids))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			existing[id] = true
		}
	}
	return existing, rows.Err()
}
