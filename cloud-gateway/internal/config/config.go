package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Enabled                                                            bool
	ServerBaseURL, ConfigEndpoint                                      string
	ConfigToken                                                        string
	ConfigPollInterval, ConfigTimeout                                  time.Duration
	RedisAddr, RequestStream, ResponseStream, DLQStream, ContextStream string
	Group, Consumer                                                    string
	BlockTimeout, RedisRetryDelay, RedisReadTimeout                    time.Duration
	Batch, Concurrency                                                 int
	ContextWindow                                                      time.Duration
	ContextMaxEvents, ContextMaxTokens, RawLogMaxChars                 int
	LatencySLAMS, TokenBudget                                          int
	StreamMaxLen                                                       int64
	MySQLDSN                                                           string
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func integer(k string, d int) int {
	v, e := strconv.Atoi(env(k, ""))
	if e != nil {
		return d
	}
	return v
}
func duration(k string, d time.Duration) time.Duration {
	v, e := time.ParseDuration(env(k, ""))
	if e != nil {
		return d
	}
	return v
}
func Load() Config {
	cfg := Config{
		Enabled: integer("CLOUD_GATEWAY_ENABLED", 1) != 0, ServerBaseURL: env("SERVER_BASE_URL", "http://server:8080"), ConfigEndpoint: env("CLOUD_CONFIG_ENDPOINT", "/api/v1/cloud-config"), ConfigToken: strings.TrimSpace(env("CLOUD_GATEWAY_CONFIG_TOKEN", "")), ConfigPollInterval: duration("CLOUD_CONFIG_POLL_INTERVAL", 10*time.Second), ConfigTimeout: duration("CLOUD_CONFIG_TIMEOUT", 5*time.Second),
		RedisAddr: env("REDIS_ADDR", "redis:6379"), RequestStream: env("CLOUD_REQ_STREAM", "cotriage.events.cloud.req"), ResponseStream: env("CLOUD_RESP_STREAM", "cotriage.events.cloud.resp"), DLQStream: env("DLQ_STREAM", "cotriage.events.dlq"), ContextStream: env("CLOUD_CONTEXT_SOURCE_STREAM", "cotriage.events.inferred"), Group: env("CLOUD_GATEWAY_GROUP", "cloud-gateway-g"), Consumer: env("CLOUD_GATEWAY_CONSUMER", "cloud-gateway-1"), BlockTimeout: duration("CLOUD_GATEWAY_BLOCK_TIMEOUT", 2*time.Second), RedisRetryDelay: duration("CLOUD_GATEWAY_REDIS_RETRY_DELAY", 3*time.Second), RedisReadTimeout: duration("CLOUD_GATEWAY_REDIS_READ_TIMEOUT", 10*time.Second), Batch: integer("CLOUD_GATEWAY_BATCH", 10), Concurrency: integer("CLOUD_GATEWAY_CONCURRENCY", 2), ContextWindow: duration("CLOUD_CONTEXT_WINDOW", 5*time.Minute), ContextMaxEvents: integer("CLOUD_CONTEXT_MAX_EVENTS", 20), ContextMaxTokens: integer("CLOUD_CONTEXT_MAX_TOKENS", 3000), RawLogMaxChars: integer("CLOUD_RAW_LOG_MAX_CHARS", 2000), LatencySLAMS: integer("CLOUD_LATENCY_SLA_MS", 5000), TokenBudget: integer("CLOUD_TOKEN_BUDGET_PER_REQ", 2000), StreamMaxLen: int64(integer("CLOUD_STREAM_MAXLEN", 100000)), MySQLDSN: env("MYSQL_DSN", "cotriage:cotriage-pass@tcp(mysql.cotriage.svc.cluster.local:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC"),
	}
	if cfg.RedisReadTimeout <= cfg.BlockTimeout {
		cfg.RedisReadTimeout = cfg.BlockTimeout + 5*time.Second
	}
	return cfg
}
