package app

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/aggregation"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/config"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/coordination"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/models"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/natsstream"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/validation"
)

type fakeCollector struct {
	events []models.ClimateEvent
	err    error
}

func (f fakeCollector) FetchEvents(context.Context, string, int, string) ([]models.ClimateEvent, error) {
	return f.events, f.err
}

func TestCollectOnceStoresAggregates(t *testing.T) {
	now := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	state := &State{}
	service := Service{
		cfg: config.Config{
			Categories: []string{"wildfires"},
			Days:       30,
			Status:     "all",
			WindowSize: time.Hour,
		},
		collector: fakeCollector{events: []models.ClimateEvent{
			{ID: "1", Category: "wildfires", Latitude: 10, Longitude: 20, OccurredAt: now},
			{ID: "2", Category: "wildfires", Latitude: 12, Longitude: 21, OccurredAt: now.Add(time.Minute)},
		}},
		coordinator: coordination.NewMemoryCoordinator(),
		validator:   validation.EventValidator{},
		aggregator:  aggregation.New(time.Hour),
		publisher:   natsstream.NoopPublisher{},
		state:       state,
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if err := service.collectOnce(context.Background()); err != nil {
		t.Fatalf("collectOnce failed: %v", err)
	}
	events, aggregates := state.Snapshot()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if len(aggregates) != 1 {
		t.Fatalf("expected 1 aggregate, got %d", len(aggregates))
	}
	if aggregates[0].Count != 2 {
		t.Fatalf("expected aggregate count 2, got %d", aggregates[0].Count)
	}
}
