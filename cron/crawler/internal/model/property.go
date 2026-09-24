package model

import (
	"encoding/json"
	"time"
)

// Property represents a single real estate listing
type Property struct {
	ID               int64           `json:"id"`
	Title            string          `json:"title"`
	Summary          string          `json:"summary"`
	Price            int64           `json:"price"`
	PriceDisplay     string          `json:"price_display"`
	CategoryID       int             `json:"category_id"`
	CategoryName     string          `json:"category_name"`
	CityID           int             `json:"city_id"`
	DistrictID       int             `json:"district_id"`
	Location         string          `json:"location"`
	URL              string          `json:"url"`
	PhoneDisplay     string          `json:"phone_display"`
	PublishAt        *time.Time      `json:"publish_at"`
	Attributes       json.RawMessage `json:"attributes"`
	Covers           json.RawMessage `json:"covers"`
	LocationsDisplay json.RawMessage `json:"locations_display"`
	RawData          json.RawMessage `json:"-"`
}

// NextDataWrapper matches the Next.js __NEXT_DATA__ structure
type NextDataWrapper struct {
	Props struct {
		PageProps struct {
			Classified struct {
				Total int               `json:"total"`
				Items []json.RawMessage `json:"items"`
			} `json:"classified"`
		} `json:"pageProps"`
	} `json:"props"`
}

// Checkpoint tracks crawler progress for resumable tasks
type Checkpoint struct {
	CategoryKey string    `json:"category_key"`
	LastPage    int       `json:"last_page"`
	TotalPages  int       `json:"total_pages"`
	TotalItems  int       `json:"total_items"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
}
