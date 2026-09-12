package db

import (
	"context"
	"fmt"
	"time"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/config"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/store"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	gdb, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(10)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBPingTimeout)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return gdb, nil
}

func EnsureSchema(ctx context.Context, gdb *gorm.DB) error {
	if err := gdb.WithContext(ctx).AutoMigrate(
		&store.RouterDecisionModel{},
		&store.RouterStateSnapshotModel{},
		&store.CloudOutboxModel{},
		&store.CloudFeedbackModel{},
	); err != nil {
		return fmt.Errorf("auto migrate router tables: %w", err)
	}
	return nil
}
