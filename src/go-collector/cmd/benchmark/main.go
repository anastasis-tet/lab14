package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/client"
)

type benchmarkConfig struct {
	baseURL        string
	categories     []string
	days           int
	status         string
	iterations     int
	timeoutSeconds int
}

type benchmarkResult struct {
	Collector       string  `json:"collector"`
	BaseURL         string  `json:"base_url"`
	Categories      int     `json:"categories"`
	Iterations      int     `json:"iterations"`
	Requests        int     `json:"requests"`
	Events          int     `json:"events"`
	ElapsedSeconds  float64 `json:"elapsed_seconds"`
	EventsPerSecond float64 `json:"events_per_second"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark failed: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}

	result, err := executeBenchmark(cfg)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func parseConfig(args []string) (benchmarkConfig, error) {
	flags := flag.NewFlagSet("go-eonet-benchmark", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	baseURL := flags.String("base-url", "https://eonet.gsfc.nasa.gov/api/v3", "EONET API base URL")
	categoryCSV := flags.String("categories", "wildfires,severeStorms,drought,seaLakeIce,dustHaze", "comma-separated EONET categories")
	days := flags.Int("days", 365, "history depth in days")
	status := flags.String("status", "all", "EONET event status")
	iterations := flags.Int("iterations", 5, "number of repeated collection cycles")
	timeoutSeconds := flags.Int("timeout-seconds", 10, "HTTP timeout per request")

	if err := flags.Parse(args); err != nil {
		return benchmarkConfig{}, err
	}

	categories := splitCSV(*categoryCSV)
	if len(categories) == 0 {
		return benchmarkConfig{}, errors.New("at least one category is required")
	}
	if *days <= 0 {
		return benchmarkConfig{}, errors.New("days must be positive")
	}
	if *iterations <= 0 {
		return benchmarkConfig{}, errors.New("iterations must be positive")
	}
	if *timeoutSeconds <= 0 {
		return benchmarkConfig{}, errors.New("timeout-seconds must be positive")
	}

	return benchmarkConfig{
		baseURL:        strings.TrimRight(*baseURL, "/"),
		categories:     categories,
		days:           *days,
		status:         strings.TrimSpace(*status),
		iterations:     *iterations,
		timeoutSeconds: *timeoutSeconds,
	}, nil
}

func executeBenchmark(cfg benchmarkConfig) (benchmarkResult, error) {
	httpClient := &http.Client{Timeout: time.Duration(cfg.timeoutSeconds) * time.Second}
	collector := client.NewEONETClient(cfg.baseURL, httpClient)

	startedAt := time.Now()
	totalEvents := 0
	for iteration := 0; iteration < cfg.iterations; iteration++ {
		events, err := collectCycle(context.Background(), collector, cfg)
		if err != nil {
			return benchmarkResult{}, err
		}
		totalEvents += events
	}
	elapsed := time.Since(startedAt).Seconds()

	eventsPerSecond := 0.0
	if elapsed > 0 {
		eventsPerSecond = float64(totalEvents) / elapsed
	}

	return benchmarkResult{
		Collector:       "go-goroutines",
		BaseURL:         cfg.baseURL,
		Categories:      len(cfg.categories),
		Iterations:      cfg.iterations,
		Requests:        len(cfg.categories) * cfg.iterations,
		Events:          totalEvents,
		ElapsedSeconds:  elapsed,
		EventsPerSecond: eventsPerSecond,
	}, nil
}

func collectCycle(ctx context.Context, collector client.EONETClient, cfg benchmarkConfig) (int, error) {
	var wg sync.WaitGroup
	eventCounts := make(chan int, len(cfg.categories))
	errCh := make(chan error, len(cfg.categories))

	for _, category := range cfg.categories {
		wg.Add(1)
		go func(category string) {
			defer wg.Done()
			events, err := collector.FetchEvents(ctx, category, cfg.days, cfg.status)
			if err != nil {
				errCh <- err
				return
			}
			eventCounts <- len(events)
		}(category)
	}

	wg.Wait()
	close(eventCounts)
	close(errCh)

	for err := range errCh {
		if err != nil {
			return 0, err
		}
	}

	totalEvents := 0
	for count := range eventCounts {
		totalEvents += count
	}
	return totalEvents, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
