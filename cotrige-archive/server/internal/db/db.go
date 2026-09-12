package db

import (
	"context"
	"fmt"
	"time"

	"github.com/pangjian-pj/cotriage-dev/server/internal/config"
	"github.com/pangjian-pj/cotriage-dev/server/internal/store"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open opens a MySQL connection.
//
// Contract:
// - cfg.MySQLDSN should include parseTime=true
// - caller should Close() the returned DB.
func Open(cfg config.Config) (*gorm.DB, error) {
	gdb, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql (gorm): %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	// Keep conservative defaults; tune later.
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(10)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return gdb, nil
}

// EnsureSchema creates MVP tables if they don't exist.

func EnsureSchema(ctx context.Context, db *gorm.DB) error {
	// Note: AutoMigrate won't drop columns; safe for MVP.
	if err := db.WithContext(ctx).AutoMigrate(
		&store.EdgeProviderModel{},
		&store.ClassifierConfigModel{},
		&store.EdgeRuleModel{},
		&store.EdgeTemplateModel{},
		&store.EdgeRuleSetModel{},
		&store.EdgeTemplateSetModel{},
		&store.CloudProviderModel{},
		&store.CompetitionMetricSnapshotModel{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}
