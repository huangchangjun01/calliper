package database

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db   *gorm.DB
	tsdb *gorm.DB
)

// Config holds database connection configuration.
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	TSDBName        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	LogLevel        logger.LogLevel
}

// DefaultConfig returns a Config populated from environment variables with sensible defaults.
func DefaultConfig() Config {
	cfg := Config{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnvInt("DB_PORT", 5432),
		User:            getEnv("DB_USER", "postgres"),
		Password:        getEnv("DB_PASSWORD", "postgres"),
		DBName:          getEnv("DB_NAME", "quant_trading"),
		TSDBName:        getEnv("TSDB_NAME", ""),
		SSLMode:         getEnv("DB_SSLMODE", "disable"),
		MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 5),
		MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 2),
		ConnMaxLifetime: time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_SEC", 300)) * time.Second,
		ConnMaxIdleTime: time.Duration(getEnvInt("DB_CONN_MAX_IDLE_SEC", 60)) * time.Second,
		LogLevel:        logger.Warn,
	}

	// TSDBName defaults to same as DBName if not explicitly set
	if cfg.TSDBName == "" {
		cfg.TSDBName = cfg.DBName
	}

	return cfg
}

// dsn builds a PostgreSQL connection string from a Config.
func (c Config) dsn(dbName string) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		c.Host, c.Port, c.User, c.Password, dbName, c.SSLMode,
	)
}

// applyPoolConfig applies connection pool settings to a gorm.DB instance.
func applyPoolConfig(gdb *gorm.DB, cfg Config) {
	sqlDB, err := gdb.DB()
	if err != nil {
		log.Printf("WARNING: failed to get underlying sql.DB for pool config: %v", err)
		return
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
}

// Initialize opens connections to both the main PostgreSQL database and
// the TimescaleDB database. It should be called once at application startup.
func Initialize(cfg Config) error {
	// Main PostgreSQL connection
	mainDSN := cfg.dsn(cfg.DBName)
	gormDB, err := gorm.Open(postgres.Open(mainDSN), &gorm.Config{
		Logger: logger.Default.LogMode(cfg.LogLevel),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to main database: %w", err)
	}
	applyPoolConfig(gormDB, cfg)
	db = gormDB
	log.Printf("Connected to main database: %s@%s:%d/%s", cfg.User, cfg.Host, cfg.Port, cfg.DBName)

	// TimescaleDB connection (may be the same database or a separate one)
	tsdsn := cfg.dsn(cfg.TSDBName)
	tsGormDB, err := gorm.Open(postgres.Open(tsdsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(cfg.LogLevel),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to TimescaleDB: %w", err)
	}
	applyPoolConfig(tsGormDB, cfg)
	tsdb = tsGormDB
	log.Printf("Connected to TimescaleDB: %s@%s:%d/%s", cfg.User, cfg.Host, cfg.Port, cfg.TSDBName)

	return nil
}

// InitializeDefault initializes the database with default configuration from environment variables.
func InitializeDefault() error {
	return Initialize(DefaultConfig())
}

// GetDB returns the main PostgreSQL GORM database instance.
// Panics if Initialize has not been called.
func GetDB() *gorm.DB {
	if db == nil {
		panic("database not initialized: call database.Initialize() first")
	}
	return db
}

// GetTSDB returns the TimescaleDB GORM database instance.
// Panics if Initialize has not been called.
func GetTSDB() *gorm.DB {
	if tsdb == nil {
		panic("database not initialized: call database.Initialize() first")
	}
	return tsdb
}

// Close closes both database connections gracefully.
func Close() {
	if db != nil {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	if tsdb != nil {
		if sqlDB, err := tsdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	log.Println("Database connections closed")
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}