package database

import (
	"fmt"
	"quota-manager/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	*gorm.DB          // Main database connection (quota_manager)
	AuthDB   *gorm.DB // Auth database connection (auth)
}

func NewDB(cfg *config.Config) (*DB, error) {
	// Connect to main database (quota_manager)
	mainDSN := cfg.Database.DSN()
	mainDB, err := gorm.Open(postgres.Open(mainDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect main database: %w", err)
	}

	// Connect to auth database (auth) - read-only access
	authDSN := cfg.AuthDatabase.DSN()
	authDB, err := gorm.Open(postgres.Open(authDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect auth database: %w", err)
	}

	// Set main database connection pool
	mainSqlDB, err := mainDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get main sql.DB: %w", err)
	}
	mainSqlDB.SetMaxIdleConns(10)
	mainSqlDB.SetMaxOpenConns(100)

	// Set auth database connection pool
	authSqlDB, err := authDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth sql.DB: %w", err)
	}
	authSqlDB.SetMaxIdleConns(5)
	authSqlDB.SetMaxOpenConns(50)

	return &DB{DB: mainDB, AuthDB: authDB}, nil
}

func (db *DB) Close() error {
	// Close main database connection
	if sqlDB, err := db.DB.DB(); err == nil {
		sqlDB.Close()
	}

	// Close auth database connection
	if authSqlDB, err := db.AuthDB.DB(); err == nil {
		authSqlDB.Close()
	}

	return nil
}
