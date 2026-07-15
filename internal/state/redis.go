package state

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
}

func New(redisAddress string) *Store {
	return &Store{client: redis.NewClient(&redis.Options{Addr: redisAddress})}
}

func (s *Store) Close() error { return s.client.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.client.Ping(ctx).Err() }

// ReplacePortMappings atomically replaces the full routing snapshot so ports
// removed from configuration cannot remain as stale Redis fields.
func (s *Store) ReplacePortMappings(ctx context.Context, mappings map[int]string) error {
	_, err := s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, "lb:port_mappings")
		for port, backend := range mappings {
			pipe.HSet(ctx, "lb:port_mappings", strconv.Itoa(port), backend)
		}
		return nil
	})
	return err
}

func (s *Store) ChangeActive(ctx context.Context, service, instance string, delta int64) error {
	key := fmt.Sprintf("lb:instances:%s:%s", service, instance)
	pipe := s.client.TxPipeline()
	pipe.HIncrBy(ctx, key, "active_connections", delta)
	pipe.HSet(ctx, key, "last_seen", time.Now().UTC().Format(time.RFC3339Nano))
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) UpsertInstance(ctx context.Context, service, instance string, fields map[string]any) error {
	key := fmt.Sprintf("lb:instances:%s:%s", service, instance)
	fields["service"] = service
	fields["instance"] = instance
	fields["last_seen"] = time.Now().UTC().Format(time.RFC3339Nano)
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, key, fields)
	pipe.SAdd(ctx, "lb:instances", key)
	_, err := pipe.Exec(ctx)
	return err
}
