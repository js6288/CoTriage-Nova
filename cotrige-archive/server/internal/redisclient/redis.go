package redisclient

import (
	redis "github.com/redis/go-redis/v9"

	"github.com/pangjian-pj/cotriage-dev/server/internal/config"
)

func New(cfg config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
}
