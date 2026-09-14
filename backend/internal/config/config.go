package config

import (
	"bufio"
	"log"
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

	JWTSecret     string
	JWTExpiration int // hours

	// CORSAllowedOrigins is the exact-origin whitelist. Empty means no
	// cross-origin requests are allowed.
	CORSAllowedOrigins []string

	// RealTradingEnabled gates real-money trading. When false, every order is
	// forced to simulated regardless of request body is_real.
	RealTradingEnabled bool

	// MLAPIKey is the shared secret sent as X-ML-API-Key to the ML service.
	MLAPIKey string

	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string

	// Evaluation / decision-support thresholds (go/no-go gates)
	PredHighConfidenceThreshold  int // high-confidence cutoff for dashboard highlights
	EvalAccuracySuspendThreshold int // 30d accuracy below this => suspend display
	EvalRetrainThreshold         int // consecutive days below this => trigger retrain
}

// LoadConfig reads configuration from environment variables and optionally a .env file.
func LoadConfig() *Config {
	loadEnvFile(".env")

	cfg := &Config{
		ServerPort: getEnv("SERVER_PORT", "8080"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "calliper"),
		DBPassword: getEnv("DB_PASSWORD", "10010hcj"),
		DBName:     getEnv("DB_NAME", "calliper_trading"),

		TSDBHost:     getEnv("TSDB_HOST", "localhost"),
		TSDBPort:     getEnv("TSDB_PORT", "5433"),
		TSDBUser:     getEnv("TSDB_USER", "calliper"),
		TSDBPassword: getEnv("TSDB_PASSWORD", "10010hcj"),
		TSDBName:     getEnv("TSDB_NAME", "calliper_tsdb"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiration: getEnvInt("JWT_EXPIRATION_HOURS", 24),

		CORSAllowedOrigins: getEnvCorsList("CORS_ALLOWED_ORIGINS",
			"http://localhost:5173,http://localhost:3000,http://127.0.0.1:5173"),

		RealTradingEnabled: getEnvBool("REAL_TRADING_ENABLED", false),

		MLAPIKey: getEnv("ML_API_KEY", ""),

		MinIOEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin123"),

		PredHighConfidenceThreshold:  getEnvInt("EVAL_HIGH_CONFIDENCE_THRESHOLD", 60),
		EvalAccuracySuspendThreshold: getEnvInt("EVAL_ACCURACY_SUSPEND_THRESHOLD", 45),
		EvalRetrainThreshold:         getEnvInt("EVAL_RETRAIN_THRESHOLD", 60),
	}

	// Startup warnings for insecure/placeholder values.
	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		log.Printf("ERROR: JWT_SECRET is empty or the default placeholder; set a strong secret explicitly in production")
	}
	if cfg.DBPassword == "" || cfg.DBPassword == "10010hcj" {
		log.Printf("WARN: DB_PASSWORD is empty/default; set a strong password explicitly in production")
	}
	if cfg.TSDBPassword == "" || cfg.TSDBPassword == "10010hcj" {
		log.Printf("WARN: TSDB_PASSWORD is empty/default; set a strong password explicitly in production")
	}
	if cfg.MinIOSecretKey == "" || cfg.MinIOSecretKey == "minioadmin123" {
		log.Printf("WARN: MINIO_SECRET_KEY is empty/default; set a strong secret explicitly in production")
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

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

// getEnvCorsList reads a comma-separated env var into an origin whitelist.
func getEnvCorsList(key, defaultVal string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		raw = defaultVal
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
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
