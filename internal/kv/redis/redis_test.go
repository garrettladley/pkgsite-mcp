package redis

import (
	"testing"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

func TestApplyOptions(t *testing.T) {
	t.Parallel()

	cfg := config.KV{
		RedisPool: config.RedisPool{
			Size:           4,
			MinIdleConns:   2,
			MaxIdleConns:   4,
			MaxActiveConns: 8,
			Timeout:        250 * time.Millisecond,
		},
		RedisTimeouts: config.RedisTimeouts{
			Dial:  time.Second,
			Read:  750 * time.Millisecond,
			Write: 750 * time.Millisecond,
		},
		RedisConnMaxIdle:     10 * time.Minute,
		RedisDisableIdentity: true,
	}
	opts := &goredis.Options{}

	applyOptions(opts, cfg)

	if !opts.DisableIdentity {
		t.Fatal("DisableIdentity = false, want true")
	}
	if opts.PoolSize != 4 {
		t.Fatalf("PoolSize = %d, want 4", opts.PoolSize)
	}
	if opts.MinIdleConns != 2 {
		t.Fatalf("MinIdleConns = %d, want 2", opts.MinIdleConns)
	}
	if opts.MaxIdleConns != 4 {
		t.Fatalf("MaxIdleConns = %d, want 4", opts.MaxIdleConns)
	}
	if opts.MaxActiveConns != 8 {
		t.Fatalf("MaxActiveConns = %d, want 8", opts.MaxActiveConns)
	}
	if opts.PoolTimeout != 250*time.Millisecond {
		t.Fatalf("PoolTimeout = %s, want 250ms", opts.PoolTimeout)
	}
	if opts.DialTimeout != time.Second {
		t.Fatalf("DialTimeout = %s, want 1s", opts.DialTimeout)
	}
	if opts.ReadTimeout != 750*time.Millisecond {
		t.Fatalf("ReadTimeout = %s, want 750ms", opts.ReadTimeout)
	}
	if opts.WriteTimeout != 750*time.Millisecond {
		t.Fatalf("WriteTimeout = %s, want 750ms", opts.WriteTimeout)
	}
	if opts.ConnMaxIdleTime != 10*time.Minute {
		t.Fatalf("ConnMaxIdleTime = %s, want 10m", opts.ConnMaxIdleTime)
	}
}
