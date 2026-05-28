package validation

import (
	"testing"
	"time"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/models"
)

func TestEventValidatorAcceptsValidEvent(t *testing.T) {
	event := models.ClimateEvent{
		ID:         "EONET_1",
		Category:   "wildfires",
		Latitude:   55.7,
		Longitude:  37.6,
		OccurredAt: time.Now().UTC(),
	}

	if err := (EventValidator{}).Validate(event); err != nil {
		t.Fatalf("expected valid event, got %v", err)
	}
}

func TestEventValidatorRejectsInvalidCoordinates(t *testing.T) {
	cases := []models.ClimateEvent{
		{ID: "1", Category: "wildfires", Latitude: -91, Longitude: 0, OccurredAt: time.Now().UTC()},
		{ID: "2", Category: "wildfires", Latitude: 0, Longitude: 181, OccurredAt: time.Now().UTC()},
	}

	for _, event := range cases {
		if err := (EventValidator{}).Validate(event); err == nil {
			t.Fatalf("expected validation error for event %+v", event)
		}
	}
}
