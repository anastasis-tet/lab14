package validation

import (
	"errors"
	"fmt"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/models"
)

type Validator interface {
	Validate(event models.ClimateEvent) error
}

type EventValidator struct{}

func (EventValidator) Validate(event models.ClimateEvent) error {
	if event.ID == "" {
		return errors.New("event id is required")
	}
	if event.Category == "" {
		return errors.New("event category is required")
	}
	if event.Latitude < -90 || event.Latitude > 90 {
		return fmt.Errorf("latitude %.4f is out of range", event.Latitude)
	}
	if event.Longitude < -180 || event.Longitude > 180 {
		return fmt.Errorf("longitude %.4f is out of range", event.Longitude)
	}
	if event.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	return nil
}
