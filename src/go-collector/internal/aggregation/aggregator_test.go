package aggregation

import (
	"testing"
	"time"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/models"
)

func TestAggregateGroupsByWindowAndCategory(t *testing.T) {
	base := time.Date(2026, 5, 28, 10, 10, 0, 0, time.UTC)
	events := []models.ClimateEvent{
		{ID: "1", Category: "wildfires", Latitude: 10, Longitude: 20, OccurredAt: base},
		{ID: "2", Category: "wildfires", Latitude: 20, Longitude: 21, OccurredAt: base.Add(10 * time.Minute)},
		{ID: "3", Category: "severeStorms", Latitude: -5, Longitude: 22, OccurredAt: base},
		{ID: "4", Category: "wildfires", Latitude: 40, Longitude: 23, OccurredAt: base.Add(2 * time.Hour)},
	}

	result := New(time.Hour).Aggregate(events)

	if len(result) != 3 {
		t.Fatalf("expected 3 aggregates, got %d", len(result))
	}
	for _, aggregate := range result {
		if aggregate.Count <= 0 {
			t.Fatalf("aggregate count must be positive: %+v", aggregate)
		}
		if aggregate.AvgLatitude < aggregate.MinLatitude || aggregate.AvgLatitude > aggregate.MaxLatitude {
			t.Fatalf("average latitude must be inside min/max: %+v", aggregate)
		}
		if !aggregate.WindowEnd.After(aggregate.WindowStart) {
			t.Fatalf("window end must be after start: %+v", aggregate)
		}
	}
}

func TestAggregateEmptyInput(t *testing.T) {
	result := New(time.Hour).Aggregate(nil)
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d", len(result))
	}
}
