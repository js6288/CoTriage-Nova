package worker

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"

	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/config"
	"github.com/redis/go-redis/v9"
)

func TestRunRetriesRedisConnectionErrorsUntilContextCanceled(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 10 * time.Millisecond,
		MaxRetries:  0,
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	w := &Worker{
		Cfg: config.Config{
			RequestStream:   "test.cloud.requests",
			Group:           "test-cloud-gateway",
			Consumer:        "test-consumer",
			RedisRetryDelay: 5 * time.Millisecond,
		},
		Redis:  rdb,
		Logger: log.New(io.Discard, "", 0),
	}

	err := w.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context deadline exceeded", err)
	}
}

func TestWaitForRetryStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := waitForRetry(ctx, time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForRetry() error = %v, want context canceled", err)
	}
}
