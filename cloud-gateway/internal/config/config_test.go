package config

import (
	"testing"
	"time"
)

func TestLoadKeepsRedisReadTimeoutAboveBlockTimeout(t *testing.T) {
	t.Setenv("CLOUD_GATEWAY_BLOCK_TIMEOUT", "12s")
	t.Setenv("CLOUD_GATEWAY_REDIS_READ_TIMEOUT", "10s")

	cfg := Load()
	if cfg.RedisReadTimeout != 17*time.Second {
		t.Fatalf("RedisReadTimeout = %s, want 17s", cfg.RedisReadTimeout)
	}
}
