package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/config"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/db"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/feedback"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/raise"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/replayer"
	routerRuntime "github.com/pangjian-pj/cotriage-dev/router-controller/internal/runtime"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/state"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/store"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/worker"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gdb, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("[router-controller] open db: %v", err)
	}
	if cfg.AutoMigrate {
		migrationTimeout := config.GetenvDuration("ROUTER_MIGRATION_TIMEOUT", 5*time.Minute)
		migrateCtx, cancel := context.WithTimeout(ctx, migrationTimeout)
		if err := db.EnsureSchema(migrateCtx, gdb); err != nil {
			cancel()
			log.Fatalf("[router-controller] ensure schema: %v", err)
		}
		cancel()
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	pingCtx, cancel := context.WithTimeout(ctx, cfg.RedisPingTimeout)
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		cancel()
		log.Fatalf("[router-controller] ping redis: %v", err)
	}
	cancel()

	log.Printf("[router-controller] ready redis=%s inferred=%s routed=%s cloud_req=%s group=%s consumer=%s",
		cfg.RedisAddr, cfg.InferredStream, cfg.RoutedStream, cfg.CloudReqStream, cfg.ConsumerGroup, cfg.ConsumerName)

	st := store.New(gdb)
	manager := loadStateManager(ctx, cfg, st)
	queueWatcher := &routerRuntime.QueueWatcher{
		Redis: rdb, Store: st, InferredStream: cfg.InferredStream, ConsumerGroup: cfg.ConsumerGroup, CloudReqStream: cfg.CloudReqStream,
	}
	runtimeBuilder := routerRuntime.NewBuilder(cfg, queueWatcher, nil, nil)
	engine := raise.New(raise.LoadConfigFromEnv())
	mainWorker := worker.New(cfg, rdb, st, engine, manager, runtimeBuilder)
	go mainWorker.Run(ctx)

	if cfg.OutboxReplayerEnabled {
		publisher := replayer.RedisPublisher{Client: rdb, Stream: cfg.CloudReqStream}
		rep := replayer.New(cfg, st, publisher)
		go rep.Run(ctx)
	} else {
		log.Printf("[router-controller] outbox replayer disabled")
	}

	if cfg.FeedbackProcessorEnabled {
		processor := feedback.New(cfg, st, manager)
		go processor.Run(ctx)
	} else {
		log.Printf("[router-controller] feedback processor disabled")
	}

	<-ctx.Done()
	log.Printf("[router-controller] exit")
}

func loadStateManager(ctx context.Context, cfg config.Config, st *store.Store) *state.Manager {
	manager := state.NewManager(cfg.StateWindow, 7, cfg.GainUCBAlpha)
	snapshot, err := st.LatestStateSnapshot(ctx, state.DefaultSnapshotType, state.DefaultSnapshotKey)
	if err != nil {
		if err != store.ErrNotFound {
			log.Printf("[router-controller] load state snapshot failed: %v", err)
		}
		return manager
	}
	restored, err := state.RestoreManager(snapshot.PayloadJSON)
	if err != nil {
		log.Printf("[router-controller] restore state snapshot failed: %v", err)
		return manager
	}
	log.Printf("[router-controller] restored state snapshot id=%d version=%s", snapshot.ID, snapshot.Version)
	return restored
}
