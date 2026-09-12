package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RedisAddr         string
	RawStream         string
	InferredStream    string
	DLQStream         string
	ConsumerGroup     string
	ConsumerName      string
	RulesFile         string
	TemplatesFile     string
	EdgeConfigURL     string
	ClassifierURL     string
	LLMURL            string
	TemplateCacheTTL  time.Duration
	ConfigRefresh     time.Duration
	ConfigTimeout     time.Duration
	ClassifierTimeout time.Duration
	LLMTimeout        time.Duration
	BlockTimeout      time.Duration
	ClaimIdleTime     time.Duration
	MaxAttempts       int64
	LogMaxBytes       int
	RawMaxLen         int64
	InferredMaxLen    int64
	DLQMaxLen         int64
	RedisRetryDelay   time.Duration
	RedisCleanupEvery time.Duration
}

func Load() (Config, error) {
	hostname, _ := os.Hostname()
	cfg := Config{
		RedisAddr:         getenv("REDIS_ADDR", "redis:6379"),
		RawStream:         getenv("RAW_STREAM", "cotriage.events.raw"),
		InferredStream:    getenv("INFERRED_STREAM", "cotriage.events.inferred"),
		DLQStream:         getenv("DLQ_STREAM", "cotriage.events.dlq"),
		ConsumerGroup:     getenv("CONSUMER_GROUP", "edge-infer-g"),
		ConsumerName:      getenv("CONSUMER_NAME", hostname),
		RulesFile:         strings.TrimSpace(os.Getenv("RULES_FILE")),
		TemplatesFile:     strings.TrimSpace(os.Getenv("TEMPLATES_FILE")),
		EdgeConfigURL:     strings.TrimSpace(os.Getenv("EDGE_CONFIG_URL")),
		ClassifierURL:     strings.TrimSpace(os.Getenv("CLASSIFIER_URL")),
		LLMURL:            strings.TrimSpace(os.Getenv("LLM_URL")),
		TemplateCacheTTL:  5 * time.Minute,
		ConfigRefresh:     30 * time.Second,
		ConfigTimeout:     3 * time.Second,
		ClassifierTimeout: 1500 * time.Millisecond,
		LLMTimeout:        3 * time.Second,
		BlockTimeout:      2 * time.Second,
		ClaimIdleTime:     30 * time.Second,
		MaxAttempts:       3,
		LogMaxBytes:       8192,
		RawMaxLen:         100000,
		InferredMaxLen:    100000,
		DLQMaxLen:         10000,
		RedisRetryDelay:   3 * time.Second,
		RedisCleanupEvery: time.Minute,
	}

	var err error
	if cfg.TemplateCacheTTL, err = durationEnv("TEMPLATE_CACHE_TTL", cfg.TemplateCacheTTL); err != nil {
		return Config{}, err
	}
	if cfg.ConfigRefresh, err = durationEnv("EDGE_CONFIG_REFRESH_INTERVAL", cfg.ConfigRefresh); err != nil {
		return Config{}, err
	}
	if cfg.ConfigTimeout, err = durationEnv("EDGE_CONFIG_TIMEOUT", cfg.ConfigTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ClassifierTimeout, err = durationEnv("CLASSIFIER_TIMEOUT", cfg.ClassifierTimeout); err != nil {
		return Config{}, err
	}
	if cfg.LLMTimeout, err = durationEnv("LLM_TIMEOUT", cfg.LLMTimeout); err != nil {
		return Config{}, err
	}
	if cfg.BlockTimeout, err = durationEnv("BLOCK_TIMEOUT", cfg.BlockTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ClaimIdleTime, err = durationEnv("CLAIM_IDLE_TIME", cfg.ClaimIdleTime); err != nil {
		return Config{}, err
	}
	if cfg.RedisRetryDelay, err = durationEnv("REDIS_RETRY_DELAY", cfg.RedisRetryDelay); err != nil {
		return Config{}, err
	}
	if cfg.RedisCleanupEvery, err = durationEnv("REDIS_CLEANUP_EVERY", cfg.RedisCleanupEvery); err != nil {
		return Config{}, err
	}
	if cfg.MaxAttempts, err = int64Env("MAX_ATTEMPTS", cfg.MaxAttempts); err != nil {
		return Config{}, err
	}
	logMaxBytes, err := int64Env("LOG_MAX_BYTES", int64(cfg.LogMaxBytes))
	if err != nil {
		return Config{}, err
	}
	cfg.LogMaxBytes = int(logMaxBytes)
	if cfg.RawMaxLen, err = int64Env("RAW_STREAM_MAXLEN", cfg.RawMaxLen); err != nil {
		return Config{}, err
	}
	if cfg.InferredMaxLen, err = int64Env("INFERRED_STREAM_MAXLEN", cfg.InferredMaxLen); err != nil {
		return Config{}, err
	}
	if cfg.DLQMaxLen, err = int64Env("DLQ_STREAM_MAXLEN", cfg.DLQMaxLen); err != nil {
		return Config{}, err
	}

	if cfg.ConsumerName == "" {
		return Config{}, fmt.Errorf("CONSUMER_NAME must not be empty")
	}
	if cfg.MaxAttempts < 1 {
		return Config{}, fmt.Errorf("MAX_ATTEMPTS must be at least 1")
	}
	if cfg.LogMaxBytes < 128 {
		return Config{}, fmt.Errorf("LOG_MAX_BYTES must be at least 128")
	}
	if cfg.RawMaxLen < 1 || cfg.InferredMaxLen < 1 || cfg.DLQMaxLen < 1 {
		return Config{}, fmt.Errorf("stream MAXLEN values must be at least 1")
	}
	if cfg.RedisRetryDelay <= 0 {
		return Config{}, fmt.Errorf("REDIS_RETRY_DELAY must be positive")
	}
	if cfg.RedisCleanupEvery <= 0 {
		return Config{}, fmt.Errorf("REDIS_CLEANUP_EVERY must be positive")
	}
	if cfg.EdgeConfigURL != "" && cfg.ConfigRefresh <= 0 {
		return Config{}, fmt.Errorf("EDGE_CONFIG_REFRESH_INTERVAL must be positive")
	}
	if cfg.EdgeConfigURL != "" && cfg.ConfigTimeout <= 0 {
		return Config{}, fmt.Errorf("EDGE_CONFIG_TIMEOUT must be positive")
	}
	if cfg.ClassifierURL != "" && cfg.ClassifierTimeout <= 0 {
		return Config{}, fmt.Errorf("CLASSIFIER_TIMEOUT must be positive")
	}
	if cfg.LLMURL != "" && cfg.LLMTimeout <= 0 {
		return Config{}, fmt.Errorf("LLM_TIMEOUT must be positive")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return duration, nil
}

func int64Env(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return number, nil
}
