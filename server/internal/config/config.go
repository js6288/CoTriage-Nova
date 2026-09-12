package config

import (
	"strings"
	"time"
)

type Config struct {
	ListenAddr string

	RedisAddr       string
	RedisStream     string
	InferredStream  string
	RoutedStream    string
	CloudReqStream  string
	CloudRespStream string
	DLQStream       string

	MySQLDSN string

	Kubeconfig string

	DevicesDefaultLimit int
	DevicesMaxLimit     int

	EventsLookback             time.Duration
	DevicesNamespace           string
	CollectorLabelSelector     string
	CompetitionMetricsInterval time.Duration
}

func Load() Config {
	return Config{
		ListenAddr: Getenv("LISTEN_ADDR", ":8080"),

		RedisAddr:       Getenv("REDIS_ADDR", "redis:6379"),
		RedisStream:     Getenv("REDIS_STREAM", "cotriage.events.raw"),
		InferredStream:  Getenv("INFERRED_STREAM", "cotriage.events.inferred"),
		RoutedStream:    Getenv("ROUTED_STREAM", "cotriage.events.routed"),
		CloudReqStream:  Getenv("CLOUD_REQ_STREAM", "cotriage.events.cloud.req"),
		CloudRespStream: Getenv("CLOUD_RESP_STREAM", "cotriage.events.cloud.resp"),
		DLQStream:       Getenv("DLQ_STREAM", "cotriage.events.dlq"),

		MySQLDSN: Getenv("MYSQL_DSN", "cotriage:cotriage-pass@tcp(mysql.cotriage.svc.cluster.local:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC"),

		Kubeconfig: strings.TrimSpace(Getenv("KUBECONFIG", "")),

		DevicesDefaultLimit: GetenvInt("DEVICES_DEFAULT_LIMIT", 200),
		DevicesMaxLimit:     GetenvInt("DEVICES_MAX_LIMIT", 1000),

		EventsLookback:             GetenvDuration("EVENTS_LOOKBACK", 60*time.Second),
		DevicesNamespace:           Getenv("COTRIAGE_NAMESPACE", "cotriage"),
		CollectorLabelSelector:     Getenv("COLLECTOR_LABEL_SELECTOR", "app=cotriage-log-collector"),
		CompetitionMetricsInterval: GetenvDuration("COMPETITION_METRICS_INTERVAL", 30*time.Second),
	}
}
