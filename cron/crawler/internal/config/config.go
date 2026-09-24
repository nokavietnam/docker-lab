package config

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL          string
	BaseURL              string
	CategoryPath         string
	StartPage            int
	MaxPages             int
	Concurrency          int
	MinDelay             time.Duration
	MaxDelay             time.Duration
	BatchSize            int
	Proxies              []string
	RunMode              string // "cron" or "once"
	CronStartHour        int    // 3 AM
	CronEndHour          int    // 5 AM
	IncrementalMode      bool   // Stop crawling early when already-stored items are reached
	MaxExistingThreshold int    // Number of consecutive existing items before stopping
}

// Load loads configurations from .env (if present) and OS environment variables.
// OS environment variables always take precedence over .env file values.
func Load() *Config {
	loadEnvFile(".env", "../.env")

	cfg := &Config{
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://lab_user:lab_password@localhost:5432/muaban_db?sslmode=disable"),
		BaseURL:              getEnv("BASE_URL", "https://muaban.net"),
		CategoryPath:         getEnv("CATEGORY_PATH", "bat-dong-san/ban-nha"),
		StartPage:            getEnvInt("START_PAGE", 1),
		MaxPages:             getEnvInt("MAX_PAGES", 500),
		Concurrency:          getEnvInt("CONCURRENCY", 2),
		MinDelay:             time.Duration(getEnvInt("MIN_DELAY_MS", 1200)) * time.Millisecond,
		MaxDelay:             time.Duration(getEnvInt("MAX_DELAY_MS", 2500)) * time.Millisecond,
		BatchSize:            getEnvInt("BATCH_SIZE", 20),
		RunMode:              getEnv("RUN_MODE", "cron"),
		CronStartHour:        getEnvInt("CRON_START_HOUR", 3),
		CronEndHour:          getEnvInt("CRON_END_HOUR", 5),
		IncrementalMode:      getEnvBool("INCREMENTAL_MODE", true),
		MaxExistingThreshold: getEnvInt("MAX_EXISTING_THRESHOLD", 20),
	}

	proxyList := os.Getenv("PROXIES")
	if proxyList != "" {
		for _, p := range strings.Split(proxyList, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				cfg.Proxies = append(cfg.Proxies, p)
			}
		}
	}

	return cfg
}

// loadEnvFile reads one or more .env files and sets environment variables if not already defined.
func loadEnvFile(filenames ...string) {
	for _, filename := range filenames {
		f, err := os.Open(filename)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "export ") {
				line = strings.TrimSpace(line[7:])
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])

			// Strip surrounding double or single quotes
			if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
				val = val[1 : len(val)-1]
			}

			// Only set if not already set in OS environment
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, val)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[Config] Warning: error scanning env file %s: %v", filename, err)
		}
		_ = f.Close()
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvBool(key string, defaultVal bool) bool {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}
