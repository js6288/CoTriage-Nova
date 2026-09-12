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

func TestEdgeKnowledgeStore_SeedBuiltinRulesIdempotent(t *testing.T) {
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

	if err := gdb.WithContext(ctx).Migrator().DropTable(&EdgeRuleSetModel{}, &EdgeRuleModel{}); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if err := gdb.WithContext(ctx).AutoMigrate(&EdgeRuleModel{}, &EdgeRuleSetModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewEdgeKnowledgeStore(gdb)
	inserted, activated, err := store.SeedBuiltinRules(ctx)
	if err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if inserted != len(builtinRuleRequests()) {
		t.Fatalf("inserted = %d, want %d", inserted, len(builtinRuleRequests()))
	}
	if !activated {
		t.Fatal("first seed should activate builtin rule set")
	}

	inserted, activated, err = store.SeedBuiltinRules(ctx)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if inserted != 0 || activated {
		t.Fatalf("second seed inserted=%d activated=%t, want 0 false", inserted, activated)
	}

	active, err := store.ActiveRules(ctx)
	if err != nil {
		t.Fatalf("active rules: %v", err)
	}
	if len(active.Items) != len(builtinRuleRequests()) {
		t.Fatalf("active rules = %d, want %d", len(active.Items), len(builtinRuleRequests()))
	}
	if active.Version == 0 || active.Etag == "" {
		t.Fatalf("active rule set should have version and etag")
	}

	items, total, err := store.ListRules(ctx, ListKnowledgeQuery{Status: string(model.KnowledgeStatusPublished), PageSize: 100})
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if total != len(builtinRuleRequests()) || len(items) != len(builtinRuleRequests()) {
		t.Fatalf("listed total=%d len=%d, want %d", total, len(items), len(builtinRuleRequests()))
	}
}
