package kv

import (
	"context"
	"time"
)

var _ Store = NoopStore{}

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
