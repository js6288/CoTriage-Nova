package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/config"
	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/contextlog"
	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/provider"
	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/store"
	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/worker"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()
	if !cfg.Enabled {
		log.Print("[cloud-gateway] disabled")
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	rdb := redis.NewClient(&redis.Options{
		Addr:        cfg.RedisAddr,
		ReadTimeout: cfg.RedisReadTimeout,
	})
	defer rdb.Close()
	db, e := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{TranslateError: true})
	if e != nil {
		log.Fatalf("open mysql: %v", e)
	}
	pm := provider.NewManager(cfg.ServerBaseURL+cfg.ConfigEndpoint, cfg.ConfigTimeout, cfg.ConfigToken)
	if e = pm.Refresh(ctx); e != nil {
		log.Printf("[cloud-gateway] initial config: %v", e)
	}
	go func() {
		t := time.NewTicker(cfg.ConfigPollInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if e := pm.Refresh(ctx); e != nil {
					log.Printf("[cloud-gateway] refresh config: %v", e)
				}
			}
		}
	}()
	w := &worker.Worker{Cfg: cfg, Redis: rdb, Store: &store.Store{DB: db}, Providers: pm, Aggregator: contextlog.Aggregator{Redis: rdb, Stream: cfg.ContextStream, Window: cfg.ContextWindow, MaxEvents: cfg.ContextMaxEvents, MaxTokens: cfg.ContextMaxTokens}}
	log.Printf("[cloud-gateway] start stream=%s group=%s consumer=%s", cfg.RequestStream, cfg.Group, cfg.Consumer)
	if e = w.Run(ctx); e != nil && !errors.Is(e, context.Canceled) {
		log.Printf("[cloud-gateway] exit: %v", e)
		os.Exit(1)
	}
}
