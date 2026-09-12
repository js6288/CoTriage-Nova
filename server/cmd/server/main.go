package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pangjian-pj/cotriage-dev/server/internal/api"
	"github.com/pangjian-pj/cotriage-dev/server/internal/config"
	"github.com/pangjian-pj/cotriage-dev/server/internal/db"
	"github.com/pangjian-pj/cotriage-dev/server/internal/kube"
	"github.com/pangjian-pj/cotriage-dev/server/internal/redisclient"
	"github.com/pangjian-pj/cotriage-dev/server/internal/store"
	"gorm.io/gorm"
)

func main() {
	// Force UTC timezone for consistent time calculations between local runs and cluster (which is typically UTC).
	// Note: time.Now() already returns an absolute time, but forcing Local avoids surprises when formatting/parsing.
	_ = os.Setenv("TZ", "UTC")
	time.Local = time.UTC

	cfg := config.Load()

	var gdb *gorm.DB
	var edgeStore *store.EdgeProvidersStore
	var cloudStore *store.CloudProvidersStore
	var classifierStore *store.ClassifierConfigsStore
	var knowledgeStore *store.EdgeKnowledgeStore
	var routerStore *store.RouterStore
	if strings.TrimSpace(cfg.MySQLDSN) != "" {
		dbConn, err := db.Open(cfg)
		if err != nil {
			log.Fatalf("init mysql: %v", err)
		}
		gdb = dbConn
		edgeStore = store.NewEdgeProvidersStore(gdb)
		cloudStore = store.NewCloudProvidersStore(gdb)
		classifierStore = store.NewClassifierConfigsStore(gdb)
		knowledgeStore = store.NewEdgeKnowledgeStore(gdb)
		routerStore = store.NewRouterStore(gdb)

		migrationTimeout := config.GetenvDuration("SERVER_MIGRATION_TIMEOUT", 5*time.Minute)
		migrateCtx, migrateCancel := context.WithTimeout(context.Background(), migrationTimeout)
		if err := db.EnsureSchema(migrateCtx, gdb); err != nil {
			migrateCancel()
			log.Fatalf("ensure schema: %v", err)
		}
		migrateCancel()

		seedTimeout := config.GetenvDuration("SERVER_SEED_TIMEOUT", 2*time.Minute)
		seedCtx, seedCancel := context.WithTimeout(context.Background(), seedTimeout)
		inserted, activated, err := knowledgeStore.SeedBuiltinRules(seedCtx)
		if err != nil {
			seedCancel()
			log.Fatalf("seed builtin rules: %v", err)
		}
		seedCancel()
		if inserted > 0 || activated {
			log.Printf("[server] builtin rules seeded inserted=%d activated=%t", inserted, activated)
		}
		sqlDB, err := gdb.DB()
		if err != nil {
			log.Fatalf("get sql db: %v", err)
		}
		defer func() { _ = sqlDB.Close() }()
	}

	k, err := kube.New(cfg)
	if err != nil {
		log.Fatalf("init kube client: %v", err)
	}
	rdb := redisclient.New(cfg)
	defer func() { _ = rdb.Close() }()

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	h := &api.Handler{Cfg: cfg, Kube: k, Redis: rdb, DB: gdb, EdgeProvidersStore: edgeStore, CloudProvidersStore: cloudStore, ClassifierStore: classifierStore, EdgeKnowledgeStore: knowledgeStore, RouterStore: routerStore}
	api.RegisterRoutes(r, h)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go h.RunCompetitionMetricsCollector(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("[server] listen on %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("listen error: %v", err)
		os.Exit(1)
	}
}
