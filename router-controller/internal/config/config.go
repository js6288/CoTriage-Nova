package config

import "time"

type Config struct {
	RedisAddr string

	InferredStream string
	RoutedStream   string
	CloudReqStream string
	DLQStream      string

	ConsumerGroup string
	ConsumerName  string

	MySQLDSN string

	StateSnapshotInterval     time.Duration
	DBPingTimeout             time.Duration
	RedisPingTimeout          time.Duration
	CloudHealthURL            string
	CloudHealthTimeout        time.Duration
	CloudHealthWindow         int
	QueuePressureMax          int
	MinuteBudget              float64
	HourBudget                float64
	DayBudget                 float64
	RateLimit                 int
	BudgetRefillInterval      time.Duration
	MinCloudRatioForAudit     float64
	EstimatedCloudLatency     time.Duration
	OutboxReplayerEnabled     bool
	OutboxReplayerInterval    time.Duration
	OutboxReplayerBatch       int
	OutboxLockTTL             time.Duration
	OutboxRetryBaseDelay      time.Duration
	OutboxMaxAttempts         int
	FeedbackProcessorEnabled  bool
	FeedbackProcessorInterval time.Duration
	FeedbackProcessorBatch    int
	StateWindow               time.Duration
	GainUCBAlpha              float64
	WorkerBlockTimeout        time.Duration
	WorkerBatch               int
	StreamMaxLen              int64
	AutoMigrate               bool
}

func Load() Config {
	return Config{
		RedisAddr: Getenv("REDIS_ADDR", "redis:6379"),

		InferredStream: Getenv("INFERRED_STREAM", "cotriage.events.inferred"),
		RoutedStream:   Getenv("ROUTED_STREAM", "cotriage.events.routed"),
		CloudReqStream: Getenv("CLOUD_REQ_STREAM", "cotriage.events.cloud.req"),
		DLQStream:      Getenv("DLQ_STREAM", "cotriage.events.dlq"),

		ConsumerGroup: Getenv("CONSUMER_GROUP", "router-g"),
		ConsumerName:  Getenv("CONSUMER_NAME", "router-controller-1"),

		MySQLDSN: Getenv("MYSQL_DSN", "cotriage:cotriage-pass@tcp(mysql.cotriage.svc.cluster.local:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC"),

		StateSnapshotInterval:     GetenvDuration("STATE_SNAPSHOT_INTERVAL", 1*time.Minute),
		DBPingTimeout:             GetenvDuration("DB_PING_TIMEOUT", 4*time.Second),
		RedisPingTimeout:          GetenvDuration("REDIS_PING_TIMEOUT", 2*time.Second),
		CloudHealthURL:            Getenv("CLOUD_HEALTH_URL", ""),
		CloudHealthTimeout:        GetenvDuration("CLOUD_HEALTH_TIMEOUT", 2*time.Second),
		CloudHealthWindow:         GetenvInt("CLOUD_HEALTH_WINDOW", 20),
		QueuePressureMax:          GetenvInt("QUEUE_PRESSURE_MAX", 1000),
		MinuteBudget:              GetenvFloat("CLOUD_BUDGET_MINUTE", 60),
		HourBudget:                GetenvFloat("CLOUD_BUDGET_HOUR", 1000),
		DayBudget:                 GetenvFloat("CLOUD_BUDGET_DAY", 10000),
		RateLimit:                 GetenvInt("CLOUD_RATE_LIMIT", 60),
		BudgetRefillInterval:      GetenvDuration("BUDGET_REFILL_INTERVAL", time.Minute),
		MinCloudRatioForAudit:     GetenvFloat("MIN_CLOUD_RATIO_FOR_AUDIT", 0.05),
		EstimatedCloudLatency:     GetenvDuration("ESTIMATED_CLOUD_LATENCY", 1*time.Second),
		OutboxReplayerEnabled:     GetenvInt("OUTBOX_REPLAYER_ENABLED", 1) != 0,
		OutboxReplayerInterval:    GetenvDuration("OUTBOX_REPLAYER_INTERVAL", 2*time.Second),
		OutboxReplayerBatch:       GetenvInt("OUTBOX_REPLAYER_BATCH", 20),
		OutboxLockTTL:             GetenvDuration("OUTBOX_LOCK_TTL", 30*time.Second),
		OutboxRetryBaseDelay:      GetenvDuration("OUTBOX_RETRY_BASE_DELAY", 5*time.Second),
		OutboxMaxAttempts:         GetenvInt("OUTBOX_MAX_ATTEMPTS", 5),
		FeedbackProcessorEnabled:  GetenvInt("FEEDBACK_PROCESSOR_ENABLED", 1) != 0,
		FeedbackProcessorInterval: GetenvDuration("FEEDBACK_PROCESSOR_INTERVAL", 5*time.Second),
		FeedbackProcessorBatch:    GetenvInt("FEEDBACK_PROCESSOR_BATCH", 20),
		StateWindow:               GetenvDuration("STATE_WINDOW", 10*time.Minute),
		GainUCBAlpha:              GetenvFloat("GAIN_UCB_ALPHA", 0.1),
		WorkerBlockTimeout:        GetenvDuration("WORKER_BLOCK_TIMEOUT", 2*time.Second),
		WorkerBatch:               GetenvInt("WORKER_BATCH", 20),
		StreamMaxLen:              int64(GetenvInt("ROUTER_STREAM_MAXLEN", 100000)),
		AutoMigrate:               GetenvInt("AUTO_MIGRATE", 1) != 0,
	}
}
