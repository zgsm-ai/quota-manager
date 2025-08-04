package database

import (
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/utils"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	*gorm.DB          // Main database connection (quota_manager)
	AuthDB   *gorm.DB // Auth database connection (auth)
}

func NewDB(cfg *config.Config) (*DB, error) {
	// Get configured timezone
	tz := utils.GetTimezone(cfg)

	// Connect to main database (quota_manager)
	mainDSN := cfg.Database.DSN() + " TimeZone=" + tz.String()
	mainDB, err := gorm.Open(postgres.Open(mainDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().In(tz)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect main database: %w", err)
	}

	// Connect to auth database (auth) - read-only access
	authDSN := cfg.AuthDatabase.DSN() + " TimeZone=" + tz.String()
	authDB, err := gorm.Open(postgres.Open(authDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().In(tz)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect auth database: %w", err)
	}

	// Set main database connection pool
	mainSqlDB, err := mainDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get main sql.DB: %w", err)
	}
	// Optimize connection pool configuration
	mainSqlDB.SetMaxIdleConns(25)                  // Increase idle connections
	mainSqlDB.SetMaxOpenConns(200)                 // Increase max connections
	mainSqlDB.SetConnMaxLifetime(time.Hour)        // Set connection max lifetime
	mainSqlDB.SetConnMaxIdleTime(30 * time.Minute) // Set idle connection max lifetime

	// Set auth database connection pool
	authSqlDB, err := authDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth sql.DB: %w", err)
	}
	// Optimize connection pool configuration
	authSqlDB.SetMaxIdleConns(15)                  // Increase idle connections
	authSqlDB.SetMaxOpenConns(100)                 // Increase max connections
	authSqlDB.SetConnMaxLifetime(time.Hour)        // Set connection max lifetime
	authSqlDB.SetConnMaxIdleTime(30 * time.Minute) // Set idle connection max lifetime

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
