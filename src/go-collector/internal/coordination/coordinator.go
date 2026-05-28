package coordination

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Coordinator interface {
	AssignedCategories(ctx context.Context, categories []string) ([]string, error)
	Close() error
}

type MemoryCoordinator struct{}

func NewMemoryCoordinator() MemoryCoordinator {
	return MemoryCoordinator{}
}

func (MemoryCoordinator) AssignedCategories(_ context.Context, categories []string) ([]string, error) {
	return append([]string(nil), categories...), nil
}

func (MemoryCoordinator) Close() error {
	return nil
}

type EtcdCoordinator struct {
	client     *clientv3.Client
	instanceID string
	ttlSeconds int64
}

func NewEtcdCoordinator(ctx context.Context, endpoints []string, logger *slog.Logger) Coordinator {
	if len(endpoints) == 0 {
		return NewMemoryCoordinator()
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		logger.Warn("etcd unavailable, using memory coordinator", slog.String("error", err.Error()))
		return NewMemoryCoordinator()
	}

	instanceID := os.Getenv("HOSTNAME")
	if instanceID == "" {
		instanceID = fmt.Sprintf("collector-%d", time.Now().UnixNano())
	}

	coordinator := &EtcdCoordinator{client: client, instanceID: instanceID, ttlSeconds: 15}
	if err := coordinator.register(ctx); err != nil {
		logger.Warn("etcd registration failed, using memory coordinator", slog.String("error", err.Error()))
		_ = client.Close()
		return NewMemoryCoordinator()
	}
	return coordinator
}

func (c *EtcdCoordinator) AssignedCategories(ctx context.Context, categories []string) ([]string, error) {
	response, err := c.client.Get(ctx, "/lab14/collectors/", clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("read collectors from etcd: %w", err)
	}
	instances := make([]string, 0, len(response.Kvs))
	for _, kv := range response.Kvs {
		instances = append(instances, string(kv.Value))
	}
	if len(instances) == 0 {
		return categories, nil
	}

	assigned := make([]string, 0, len(categories))
	for _, category := range categories {
		idx := int(hash(category) % uint32(len(instances)))
		if instances[idx] == c.instanceID {
			assigned = append(assigned, category)
		}
	}
	if len(assigned) == 0 {
		return categories[:1], nil
	}
	return assigned, nil
}

func (c *EtcdCoordinator) Close() error {
	return c.client.Close()
}

func (c *EtcdCoordinator) register(ctx context.Context) error {
	lease, err := c.client.Grant(ctx, c.ttlSeconds)
	if err != nil {
		return fmt.Errorf("grant etcd lease: %w", err)
	}
	key := "/lab14/collectors/" + c.instanceID
	if _, err := c.client.Put(ctx, key, c.instanceID, clientv3.WithLease(lease.ID)); err != nil {
		return fmt.Errorf("register collector: %w", err)
	}
	keepAlive, err := c.client.KeepAlive(ctx, lease.ID)
	if err != nil {
		return fmt.Errorf("start keepalive: %w", err)
	}
	go func() {
		for range keepAlive {
		}
	}()
	return nil
}

func hash(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}
