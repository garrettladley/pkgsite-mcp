package kv

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("kv: key not found")

type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Increment(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

type NoopStore struct{}

func NewNoopStore() NoopStore {
	return NoopStore{}
}

func (NoopStore) Get(context.Context, string) ([]byte, error) {
	return nil, ErrNotFound
}

func (NoopStore) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (NoopStore) Increment(context.Context, string, time.Duration) (int64, error) {
	return 1, nil
}
