package config

import "testing"

func TestConfigValidationRejectsInvalidDays(t *testing.T) {
	cfg := Config{
		BaseURL:      "https://eonet.gsfc.nasa.gov/api/v3",
		Categories:   []string{"wildfires"},
		Days:         0,
		HTTPAddr:     ":8080",
		PollInterval: 1,
		WindowSize:   1,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid days error")
	}
}

func TestSplitCSVTrimsEmptyValues(t *testing.T) {
	result := splitCSV(" wildfires, , severeStorms ")
	if len(result) != 2 {
		t.Fatalf("expected 2 values, got %d", len(result))
	}
	if result[0] != "wildfires" || result[1] != "severeStorms" {
		t.Fatalf("unexpected values: %#v", result)
	}
}
