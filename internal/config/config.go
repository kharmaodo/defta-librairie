package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	DBPath         string
	Version        string
	BuildDate      string
	PageSize       int
	JWTSecret      string
	JWTIssuer      string
	JWTAudience    string
	JWTAccessTTL   time.Duration
	JWTRefreshTTL  time.Duration
	AuthRateLimit  int
	AuthRateWindow time.Duration
}

func Load() (*Config, error) {
	// Charger .env (optionnel si absent)
	_ = godotenv.Load()

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		DBPath:         getEnv("DB_PATH", "./data/defta.db"),
		Version:        getEnv("VERSION", "0.1.0-dev"),
		BuildDate:      getEnv("BUILD_DATE", "unknown"),
		PageSize:       getEnvInt("PAGE_SIZE", 30),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		JWTIssuer:      getEnv("JWT_ISSUER", "defta-librairie"),
		JWTAudience:    getEnv("JWT_AUDIENCE", "defta-librairie-web"),
		JWTAccessTTL:   time.Duration(getEnvInt("JWT_ACCESS_TTL_SECONDS", 900)) * time.Second,
		JWTRefreshTTL:  time.Duration(getEnvInt("JWT_REFRESH_TTL_SECONDS", 604800)) * time.Second,
		AuthRateLimit:  getEnvInt("AUTH_RATE_LIMIT_REQUESTS", 10),
		AuthRateWindow: time.Duration(getEnvInt("AUTH_RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	val := getEnv(key, "")
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("Valeur invalide pour %s → utilisation de %d", key, fallback)
		return fallback
	}
	return n
}
