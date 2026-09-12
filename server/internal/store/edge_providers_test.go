package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestEdgeProvidersStore_ActivateAndConfig(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN_TEST")
	if dsn == "" {
		t.Skip("set MYSQL_DSN_TEST to run integration-style mysql tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	defer sqlDB.Close()

	s := NewEdgeProvidersStore(gdb)

	// Reset schema per test.
	if err := gdb.WithContext(ctx).Migrator().DropTable(&EdgeProviderModel{}); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if err := gdb.WithContext(ctx).AutoMigrate(&EdgeProviderModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	p1, err := s.Create(ctx, model.CreateEdgeProviderRequest{
		Name:         "p1",
		Provider:     model.EdgeProviderTypeLlamaCPP,
		Endpoint:     "http://127.0.0.1:8080/v1/chat/completions",
		Model:        "default",
		TimeoutMS:    500,
		MaxTokens:    80,
		Temperature:  0,
		TopP:         1,
		ResponsePath: "choices[0].message.content",
	})
	if err != nil {
		t.Fatalf("create p1: %v", err)
	}

	p2, err := s.Create(ctx, model.CreateEdgeProviderRequest{
		Name:         "p2",
		Provider:     model.EdgeProviderTypeVLLM,
		Endpoint:     "http://127.0.0.1:8000/v1/chat/completions",
		Model:        "default",
		TimeoutMS:    500,
		MaxTokens:    80,
		Temperature:  0,
		TopP:         1,
		ResponsePath: "choices[0].message.content",
	})
	if err != nil {
		t.Fatalf("create p2: %v", err)
	}

	cfg1, err := s.Activate(ctx, p1.ID)
	if err != nil {
		t.Fatalf("activate p1: %v", err)
	}
	if cfg1.ActiveProvider == nil || cfg1.ActiveProvider.ID != p1.ID {
		t.Fatalf("expected active p1")
	}
	if cfg1.Etag == "" || cfg1.ConfigVersion == 0 {
		t.Fatalf("expected etag and config_version")
	}

	cfg2, err := s.Activate(ctx, p2.ID)
	if err != nil {
		t.Fatalf("activate p2: %v", err)
	}
	if cfg2.ActiveProvider == nil || cfg2.ActiveProvider.ID != p2.ID {
		t.Fatalf("expected active p2")
	}
	if cfg2.ConfigVersion <= cfg1.ConfigVersion {
		t.Fatalf("expected config_version to increase")
	}

	active, err := s.GetActiveConfig(ctx)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active.ActiveProvider == nil || active.ActiveProvider.ID != p2.ID {
		t.Fatalf("expected active p2")
	}
}
