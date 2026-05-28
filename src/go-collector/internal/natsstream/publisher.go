package natsstream

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/models"
	"github.com/nats-io/nats.go"
)

type Publisher interface {
	Publish(ctx context.Context, aggregate models.WindowAggregate) error
	Close()
}

type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, models.WindowAggregate) error {
	return nil
}

func (NoopPublisher) Close() {}

type NATSPublisher struct {
	conn    *nats.Conn
	subject string
}

func New(url string, subject string, logger *slog.Logger) Publisher {
	if url == "" {
		return NoopPublisher{}
	}

	conn, err := nats.Connect(url, nats.Timeout(3*time.Second), nats.Name("lab14-go-collector"))
	if err != nil {
		logger.Warn("nats unavailable, streaming disabled", slog.String("error", err.Error()))
		return NoopPublisher{}
	}
	return &NATSPublisher{conn: conn, subject: subject}
}

func (p *NATSPublisher) Publish(ctx context.Context, aggregate models.WindowAggregate) error {
	payload, err := json.Marshal(aggregate)
	if err != nil {
		return fmt.Errorf("marshal aggregate: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- p.conn.Publish(p.subject, payload)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (p *NATSPublisher) Close() {
	p.conn.Drain()
	p.conn.Close()
}
