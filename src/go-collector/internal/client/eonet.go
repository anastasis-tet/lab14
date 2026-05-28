package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/models"
)

type EONETClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewEONETClient(baseURL string, httpClient *http.Client) EONETClient {
	return EONETClient{baseURL: baseURL, httpClient: httpClient}
}

func (c EONETClient) FetchEvents(ctx context.Context, category string, days int, status string) ([]models.ClimateEvent, error) {
	endpoint, err := url.Parse(c.baseURL + "/events")
	if err != nil {
		return nil, fmt.Errorf("parse eonet url: %w", err)
	}
	query := endpoint.Query()
	query.Set("category", category)
	query.Set("days", fmt.Sprintf("%d", days))
	query.Set("status", status)
	query.Set("limit", "500")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create eonet request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch eonet events: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("eonet returned status %d", response.StatusCode)
	}

	var payload eonetResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode eonet response: %w", err)
	}

	now := time.Now().UTC()
	events := make([]models.ClimateEvent, 0, len(payload.Events))
	for _, event := range payload.Events {
		converted, ok := convertEvent(event, category, now)
		if ok {
			events = append(events, converted)
		}
	}
	return events, nil
}

type eonetResponse struct {
	Events []eonetEvent `json:"events"`
}

type eonetEvent struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Categories []eonetCategory `json:"categories"`
	Sources    []eonetSource   `json:"sources"`
	Geometry   []eonetGeometry `json:"geometry"`
}

type eonetCategory struct {
	ID string `json:"id"`
}

type eonetSource struct {
	ID string `json:"id"`
}

type eonetGeometry struct {
	Date        time.Time       `json:"date"`
	Coordinates json.RawMessage `json:"coordinates"`
}

func convertEvent(event eonetEvent, requestedCategory string, collectedAt time.Time) (models.ClimateEvent, bool) {
	if len(event.Geometry) == 0 {
		return models.ClimateEvent{}, false
	}

	latest := event.Geometry[len(event.Geometry)-1]
	lon, lat, ok := parsePoint(latest.Coordinates)
	if !ok {
		return models.ClimateEvent{}, false
	}

	category := requestedCategory
	if len(event.Categories) > 0 && event.Categories[0].ID != "" {
		category = event.Categories[0].ID
	}

	source := "NASA-EONET"
	if len(event.Sources) > 0 && event.Sources[0].ID != "" {
		source = event.Sources[0].ID
	}

	return models.ClimateEvent{
		ID:          event.ID,
		Title:       event.Title,
		Category:    category,
		Source:      source,
		Latitude:    lat,
		Longitude:   lon,
		OccurredAt:  latest.Date.UTC(),
		CollectedAt: collectedAt,
	}, true
}

func parsePoint(raw json.RawMessage) (float64, float64, bool) {
	var point []float64
	if err := json.Unmarshal(raw, &point); err == nil && len(point) >= 2 {
		return point[0], point[1], true
	}

	var nested [][]float64
	if err := json.Unmarshal(raw, &nested); err == nil && len(nested) > 0 && len(nested[0]) >= 2 {
		return nested[0][0], nested[0][1], true
	}
	return 0, 0, false
}
