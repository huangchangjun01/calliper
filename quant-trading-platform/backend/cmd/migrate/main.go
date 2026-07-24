package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq"
)

func main() {
	// Connect to business DB (PostgreSQL)
	pgHost := getEnv("POSTGRES_HOST", "localhost")
	pgPort := getEnv("POSTGRES_PORT", "5432")
	pgUser := getEnv("POSTGRES_USER", "quant")
	pgPass := getEnv("POSTGRES_PASSWORD", "quant123")
	pgDB := getEnv("POSTGRES_DB", "quant_trading")

	pgDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		pgHost, pgPort, pgUser, pgPass, pgDB)

	pgDBConn, err := sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgDBConn.Close()

	if err := pgDBConn.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Connect to TimescaleDB
	tsHost := getEnv("TIMESCALE_HOST", "localhost")
	tsPort := getEnv("TIMESCALE_PORT", "5433")
	tsDB := getEnv("TIMESCALE_DB", "quant_tsdb")

	tsDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		tsHost, tsPort, pgUser, pgPass, tsDB)

	tsDBConn, err := sql.Open("postgres", tsDSN)
	if err != nil {
		log.Fatalf("Failed to connect to TimescaleDB: %v", err)
	}
	defer tsDBConn.Close()

	if err := tsDBConn.Ping(); err != nil {
		log.Fatalf("Failed to ping TimescaleDB: %v", err)
	}
	log.Println("Connected to TimescaleDB")

	// Run migrations
	migrationsDir := filepath.Join("migrations")
	if err := runMigrations(pgDBConn, migrationsDir, "001_init"); err != nil {
		log.Fatalf("Migration 001 failed: %v", err)
	}

	if err := runMigrations(tsDBConn, migrationsDir, "002_timescale_setup"); err != nil {
		log.Fatalf("Migration 002 failed: %v", err)
	}

	log.Println("All migrations completed successfully")
}

func runMigrations(db *sql.DB, dir, prefix string) error {
	files, err := filepath.Glob(filepath.Join(dir, prefix+"*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, f := range files {
		log.Printf("Running migration: %s", f)
		content, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", f, err)
		}

		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("failed to execute statement in %s: %w\nSQL: %s", f, err, stmt)
			}
		}
		log.Printf("Migration %s completed", f)
	}
	return nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}