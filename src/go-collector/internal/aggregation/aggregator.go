package aggregation

import (
	"sort"
	"time"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/models"
)

type Aggregator struct {
	windowSize time.Duration
}

func New(windowSize time.Duration) Aggregator {
	return Aggregator{windowSize: windowSize}
}

func (a Aggregator) Aggregate(events []models.ClimateEvent) []models.WindowAggregate {
	if len(events) == 0 {
		return nil
	}

	type key struct {
		start    time.Time
		category string
	}
	type state struct {
		count       int64
		sumLat      float64
		minLatitude float64
		maxLatitude float64
	}

	states := make(map[key]state)
	for _, event := range events {
		start := event.OccurredAt.UTC().Truncate(a.windowSize)
		k := key{start: start, category: event.Category}
		current, exists := states[k]
		if !exists {
			current.minLatitude = event.Latitude
			current.maxLatitude = event.Latitude
		}
		current.count++
		current.sumLat += event.Latitude
		current.minLatitude = min(current.minLatitude, event.Latitude)
		current.maxLatitude = max(current.maxLatitude, event.Latitude)
		states[k] = current
	}

	aggregates := make([]models.WindowAggregate, 0, len(states))
	for k, state := range states {
		aggregates = append(aggregates, models.WindowAggregate{
			WindowStart: k.start,
			WindowEnd:   k.start.Add(a.windowSize),
			Category:    k.category,
			Count:       state.count,
			MinLatitude: state.minLatitude,
			MaxLatitude: state.maxLatitude,
			AvgLatitude: state.sumLat / float64(state.count),
		})
	}

	sort.Slice(aggregates, func(i, j int) bool {
		if aggregates[i].WindowStart.Equal(aggregates[j].WindowStart) {
			return aggregates[i].Category < aggregates[j].Category
		}
		return aggregates[i].WindowStart.Before(aggregates[j].WindowStart)
	})
	return aggregates
}
