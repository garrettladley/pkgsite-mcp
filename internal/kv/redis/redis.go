package redis

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/kv"
	"github.com/redis/go-redis/extra/redisotel/v9"
	goredis "github.com/redis/go-redis/v9"
)

//go:embed ratelimit.lua
var rateLimitLua string

var rateLimitScript = goredis.NewScript(rateLimitLua)

type Store struct {
	client *goredis.Client
}

var _ kv.Store = (*Store)(nil)

func New(redisURL string) (kv.Store, error) {
	if redisURL == "" {
		return nil, nil
	}
	return NewStore(redisURL)
}

func NewStore(redisURL string) (*Store, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := goredis.NewClient(opts)
	if err := redisotel.InstrumentTracing(client); err != nil {
		return nil, err
	}
	if err := redisotel.InstrumentMetrics(client); err != nil {
		return nil, err
	}
	return &Store{client: client}, nil
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, kv.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *Store) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if ttl <= 0 {
		return 0, fmt.Errorf("increment ttl must be positive")
	}
	count, err := rateLimitScript.Run(ctx, s.client, []string{key}, ttl.Milliseconds()).Int64()
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) Close() error {
	return s.client.Close()
}
