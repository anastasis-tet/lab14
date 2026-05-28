package models

import "time"

type ClimateEvent struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Category    string    `json:"category"`
	Source      string    `json:"source"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	OccurredAt  time.Time `json:"occurred_at"`
	CollectedAt time.Time `json:"collected_at"`
}

type WindowAggregate struct {
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Category    string    `json:"category"`
	Count       int64     `json:"count"`
	MinLatitude float64   `json:"min_latitude"`
	MaxLatitude float64   `json:"max_latitude"`
	AvgLatitude float64   `json:"avg_latitude"`
}
