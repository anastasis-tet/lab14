package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/aggregation"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/arrowserver"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/client"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/config"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/coordination"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/models"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/natsstream"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/validation"
)

type Collector interface {
	FetchEvents(ctx context.Context, category string, days int, status string) ([]models.ClimateEvent, error)
}

type State struct {
	mu         sync.RWMutex
	events     []models.ClimateEvent
	aggregates []models.WindowAggregate
}

func (s *State) Replace(events []models.ClimateEvent, aggregates []models.WindowAggregate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append([]models.ClimateEvent(nil), events...)
	s.aggregates = append([]models.WindowAggregate(nil), aggregates...)
}

func (s *State) Snapshot() ([]models.ClimateEvent, []models.WindowAggregate) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]models.ClimateEvent(nil), s.events...), append([]models.WindowAggregate(nil), s.aggregates...)
}

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	collector := client.NewEONETClient(cfg.BaseURL, httpClient)
	coordinator := coordination.NewEtcdCoordinator(ctx, cfg.EtcdEndpoints, logger)
	defer coordinator.Close()
	publisher := natsstream.New(cfg.NATSURL, cfg.NATSSubject, logger)
	defer publisher.Close()

	state := &State{}
	service := Service{
		cfg:         cfg,
		collector:   collector,
		coordinator: coordinator,
		validator:   validation.EventValidator{},
		aggregator:  aggregation.New(cfg.WindowSize),
		publisher:   publisher,
		state:       state,
		logger:      logger,
	}

	server := newHTTPServer(cfg.HTTPAddr, state)
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("collector http server started", slog.String("addr", cfg.HTTPAddr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	if err := service.collectOnce(ctx); err != nil {
		logger.Warn("initial collection failed", slog.String("error", err.Error()))
	}

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return server.Shutdown(shutdownCtx)
		case err := <-serverErrors:
			return err
		case <-ticker.C:
			if err := service.collectOnce(ctx); err != nil {
				logger.Warn("collection cycle failed", slog.String("error", err.Error()))
			}
		}
	}
}

type Service struct {
	cfg         config.Config
	collector   Collector
	coordinator coordination.Coordinator
	validator   validation.Validator
	aggregator  aggregation.Aggregator
	publisher   natsstream.Publisher
	state       *State
	logger      *slog.Logger
}

func (s Service) collectOnce(ctx context.Context) error {
	categories, err := s.coordinator.AssignedCategories(ctx, s.cfg.Categories)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	eventCh := make(chan models.ClimateEvent, 256)
	errCh := make(chan error, len(categories))

	for _, category := range categories {
		wg.Add(1)
		go func(category string) {
			defer wg.Done()
			events, err := s.collector.FetchEvents(ctx, category, s.cfg.Days, s.cfg.Status)
			if err != nil {
				errCh <- err
				return
			}
			for _, event := range events {
				if err := s.validator.Validate(event); err != nil {
					s.logger.Warn("invalid climate event skipped", slog.String("event_id", event.ID), slog.String("error", err.Error()))
					continue
				}
				eventCh <- event
			}
		}(category)
	}

	go func() {
		wg.Wait()
		close(eventCh)
		close(errCh)
	}()

	events := make([]models.ClimateEvent, 0)
	for event := range eventCh {
		events = append(events, event)
	}

	aggregates := s.aggregator.Aggregate(events)
	for _, aggregate := range aggregates {
		if err := s.publisher.Publish(ctx, aggregate); err != nil {
			s.logger.Warn("publish aggregate failed", slog.String("error", err.Error()))
		}
	}
	s.state.Replace(events, aggregates)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	s.logger.Info("collection cycle completed", slog.Int("events", len(events)), slog.Int("aggregates", len(aggregates)))
	return nil
}

func newHTTPServer(addr string, state *State) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/metrics", func(writer http.ResponseWriter, request *http.Request) {
		events, aggregates := state.Snapshot()
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]int{"events": len(events), "aggregates": len(aggregates)})
	})
	mux.HandleFunc("/arrow", func(writer http.ResponseWriter, request *http.Request) {
		_, aggregates := state.Snapshot()
		writer.Header().Set("Content-Type", "application/vnd.apache.arrow.stream")
		if err := arrowserver.WriteAggregates(writer, aggregates); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
		}
	})
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
