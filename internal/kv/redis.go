package kv

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

//go:embed ratelimit.lua
var rateLimitLua string

var rateLimitScript = redis.NewScript(rateLimitLua)

type RedisStore struct {
	client *redis.Client
}

func NewStore(redisURL string) (Store, error) {
	if redisURL == "" {
		return nil, nil
	}
	return NewRedisStore(redisURL)
}

func NewRedisStore(redisURL string) (*RedisStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	if err := redisotel.InstrumentTracing(client); err != nil {
		return nil, err
	}
	if err := redisotel.InstrumentMetrics(client); err != nil {
		return nil, err
	}
	return &RedisStore{client: client}, nil
}

func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *RedisStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if ttl <= 0 {
		return 0, fmt.Errorf("increment ttl must be positive")
	}
	count, err := rateLimitScript.Run(ctx, s.client, []string{key}, ttl.Milliseconds()).Int64()
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}
