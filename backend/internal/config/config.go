package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the application.
type Config struct {
	ServerPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	TSDBHost     string
	TSDBPort     string
	TSDBUser     string
	TSDBPassword string
	TSDBName     string

	RedisHost     string
	RedisPort     string
	RedisPassword string

	KafkaBrokers string

	JWTSecret     string
	JWTExpiration int // hours

	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
}

// LoadConfig reads configuration from environment variables and optionally a .env file.
func LoadConfig() *Config {
	loadEnvFile(".env")

	cfg := &Config{
		ServerPort: getEnv("SERVER_PORT", "8080"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "quant"),
		DBPassword: getEnv("DB_PASSWORD", "quant123"),
		DBName:     getEnv("DB_NAME", "quant_trading"),

		TSDBHost:     getEnv("TSDB_HOST", "localhost"),
		TSDBPort:     getEnv("TSDB_PORT", "5433"),
		TSDBUser:     getEnv("TSDB_USER", "quant"),
		TSDBPassword: getEnv("TSDB_PASSWORD", "quant123"),
		TSDBName:     getEnv("TSDB_NAME", "quant_tsdb"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		KafkaBrokers: getEnv("KAFKA_BROKERS", ""),

		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiration: getEnvInt("JWT_EXPIRATION_HOURS", 24),

		MinIOEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin123"),
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// loadEnvFile reads a .env file and sets environment variables if not already set.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove surrounding quotes if present
		value = strings.Trim(value, `"'`)
		// Only set if not already in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}