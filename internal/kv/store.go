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
